package crypto

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/bootstrap"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/config"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/logging"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/utils"

	// "github.com/JohnnyAsh-U/ashrix-api/pkg/filehelper"
	"os"
	"path/filepath"
	"sync"

	pki_utils "github.com/JohnnyAsh-U/ashrix-api/pkg/pki"
	"go.uber.org/zap"
)

type GatewayPKI struct {
	mu       sync.RWMutex
	cert     *tls.Certificate
	privKey  *ecdsa.PrivateKey
	leaf     *x509.Certificate
	certPool *x509.CertPool

	gatewayID string
	cp        *bootstrap.Client
	log       *zap.Logger

	cfg *config.Config

	stopCh chan struct{}
	once   sync.Once
}

func NewGatewayPKI(gatewayID, dataDir, secret, context string, log *zap.Logger, cpClient *bootstrap.Client, cfg *config.Config) (*GatewayPKI, error) {
	keyPath := filepath.Join(dataDir, "gateway.key.enc")
	certPath := filepath.Join(dataDir, "gateway.crt")
	trustPath := filepath.Join(dataDir, "bundle.crt")

	// Load Identity
	key, cert, err := pki_utils.LoadKeyAndCert(keyPath, certPath, secret, context)
	if err != nil {
		return nil, fmt.Errorf("failed to load gateway identity: %w", err)
	}

	// Load Trust Bundle
	trustPEM, err := os.ReadFile(trustPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read trust bundle: %w", err)
	}

	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(trustPEM) {
		return nil, fmt.Errorf("failed to parse trust bundle")
	}

	tlsCert := &tls.Certificate{
		Certificate: [][]byte{cert.Raw},
		PrivateKey:  key,
		Leaf:        cert,
	}

	return &GatewayPKI{
		cert:      tlsCert,
		privKey:   key,
		gatewayID: gatewayID,
		leaf:      cert,
		certPool:  pool,
		cp:        cpClient,
		log:       log,
		cfg:       cfg,
		stopCh:    make(chan struct{}),
	}, nil
}

// Cert return the current certificate as *509.Certificate
func (g *GatewayPKI) Cert() *x509.Certificate {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.leaf
}

// GetTLSConfig returns a config works for inbound connection and outbound commenction
// For Gateway to CP, and Connector to Gateway
func (g *GatewayPKI) GetTLSConfig() *tls.Config {
	return &tls.Config{
		//InboundConnector dials gateway
		GetCertificate: func(chi *tls.ClientHelloInfo) (*tls.Certificate, error) {
			g.mu.RLock()
			defer g.mu.RUnlock()
			return g.cert, nil
		},
		//Outbound: gateway dials CP
		GetClientCertificate: func(cri *tls.CertificateRequestInfo) (*tls.Certificate, error) {
			g.mu.RLock()
			defer g.mu.RUnlock()
			return g.cert, nil
		},

		RootCAs:    g.certPool, //Verify CP cert
		ClientCAs:  g.certPool, //Verify Connector cert(for inbound connection)
		ClientAuth: tls.RequireAndVerifyClientCert,
		MinVersion: tls.VersionTLS13,
	}
}

// Renewals-----------------------------
//StartRotator begins the background cert rotation loop
//Call this after New(), It runs until Stop() is called

func (g *GatewayPKI) StartRotator() {
	go g.rotator()
}

// Stop shuts down the rotator. Safe to call multiple times
func (g *GatewayPKI) Stop() {
	g.once.Do(func() {
		close(g.stopCh)
	})
}

// RenewNow forces an immediate renewal attempt
// Used when the CP sends a rotation command
func (g *GatewayPKI) RenewNow() error {
	return g.renew()
}

// Rotator checks expiry every 5mins and renews when < 30 days remain
func (g *GatewayPKI) rotator() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			g.mu.RLock()
			remaining := time.Until(g.leaf.NotAfter)
			g.mu.RUnlock()
			if remaining < 30*24*time.Hour {
				g.log.Info("certificate expiring soon", zap.Duration("remaining", remaining))

				if err := g.renew(); err != nil {
					g.log.Error("cert renewal failed", zap.Error(err))
				}
			}

		case <-g.stopCh:
			g.log.Info("Cert Rotator stopped")
			return
		}
	}
}

// Renew Generates a new keypair, builds a CSR with possession proof.
// calls the CP, and hot-swaps the certificate
// The write lock is held only during the final swap - not during the network call.
// This means TLS handshakes are never blocked by a slow CP
func (g *GatewayPKI) renew() error {
	// Step 1: Read current state (read lock, fast) ------------
	g.mu.RLock()
	remaining := time.Until(g.leaf.NotAfter)
	currentPrivKey := g.privKey
	g.mu.RUnlock()

	//Another goroutines may have already renewed (e.g. RenewNow() race)
	if remaining > 90*24*time.Hour {
		g.log.Debug("Renewal Skipped - Cert still fresh", zap.Duration("remaining", remaining))
		return nil
	}

	//Create new key pair
	g.log.Info("Generating ECDSA P-256 keypair...")

	priv, err := GenerateECDSAP256()

	if err != nil {
		return fmt.Errorf("renew: keygen failed %w", err)
	}

	//Build CSR
	fmt.Println("Generating CSR...")

	csrPem, err := GenerateCSR(priv)

	if err != nil {
		return fmt.Errorf("renew: CSR generation failed %w", err)
	}

	//Call CP (no lock held here)
	g.log.Info("Calling CP for cert renewal")

	// Build Possession Proof (signs with current key)
	// Proves to the CP that we hold the private key for the currently
	// issued certificate - required per PKI
	timestamp := time.Now().UTC().Unix()
	proof, err := buildPossessionProof(currentPrivKey, csrPem, g.gatewayID, timestamp)

	if err != nil {
		return fmt.Errorf("build possession proof: %w", err)
	}

	apiResponse, err := g.cp.RenewCert(proof, string(csrPem), g.gatewayID)

	if err != nil {
		return fmt.Errorf("")
	}

	if err := bootstrap.VerifyResponse(apiResponse); err != nil {
		return fmt.Errorf("Invalid CP Response %w", err)
	}

	//Parse Cert
	newCertDER, err := base64.StdEncoding.DecodeString(apiResponse.Data.Certificate)
	if err != nil {
		return fmt.Errorf("cannot decode certificate: %v", err)
	}

	newLeaf, err := x509.ParseCertificate(newCertDER)
	if err != nil {
		return fmt.Errorf("cannot parse new certificate: %v", err)
	}

	//Parse Bundle
	newTrustPEM, err := base64.StdEncoding.DecodeString(apiResponse.Data.TrustBundle)
	if err != nil {
		return fmt.Errorf("cannot decode trust bundle: %v", err)
	}

	newPool := x509.NewCertPool()
	if !newPool.AppendCertsFromPEM(newTrustPEM) {
		return fmt.Errorf("cannot parse new trust bundle")
	}

	//Sanity check for the extended expiry
	if !newLeaf.NotAfter.After(g.leaf.NotAfter) {
		return fmt.Errorf("certificate validity is not extended, refusing")
	}

	g.log.Info("Renew Successful", zap.String("gateway_id", g.gatewayID), zap.Any("apiResponse", apiResponse))

	g.log.Info("Writing new cert, bundle and key to directory")

	Home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("Error %s", err)
	}
	configDir := filepath.Join(Home, "/.ashrix")

	writeErr := utils.WriteConfig(g.cfg.CPURL, g.gatewayID, g.cfg.GatewayName,g.cfg.TenantId, g.cfg.GatewayUrl, g.cfg.DataDir, configDir, g.cfg.LogDir)

	if writeErr != nil {
		return fmt.Errorf("write config failed: %w", writeErr)
	}

	if saveErr := utils.SaveKeyAndCertAndBundle(priv, apiResponse.Data.Certificate, apiResponse.Data.TrustBundle, g.cfg.DataDir, "SECRET", "GATEWAY"); saveErr != nil {
		return fmt.Errorf("save key and cert and bundle failed: %w", saveErr)
	}

	logging.Audit.Log(logging.AuditEvent{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		EventType: logging.EventGatewayRegistered,
		GatewayID: apiResponse.Data.GatewayId,
		Decision:  "ALLOW",
	})

	g.log.Info("Gateway Cert Renewed, Swapping", zap.String("gateway_id", apiResponse.Data.GatewayId))

	//Swapping
	g.mu.Lock()
	defer g.mu.Unlock()

	g.leaf = newLeaf
	g.privKey = priv
	g.cert = &tls.Certificate{
		Certificate: [][]byte{newLeaf.Raw},
		PrivateKey:  priv,
		Leaf:        newLeaf,
	}

	g.certPool = newPool

	g.log.Info("Gateway cert swapped", zap.String("gateway_id", g.gatewayID))

	return nil
}

// buildPossessionProof signs SHA256(csr || connectorID || timestamp_bytes)
// with the current private key.
func buildPossessionProof(
	currentKey *ecdsa.PrivateKey,
	csrPEM []byte,
	gatewayID string,
	timestamp int64,
) (string, error) {

	h := sha256.New()
	h.Write(csrPEM)
	h.Write([]byte(gatewayID))

	// Convert int64 timestamp to 8-byte big-endian slice
	timeBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(timeBytes, uint64(timestamp))
	h.Write(timeBytes)

	digest := h.Sum(nil)

	sig, err := ecdsa.SignASN1(rand.Reader, currentKey, digest)
	if err != nil {
		return "", fmt.Errorf("sign possession proof: %w", err)
	}

	// Encode as base64 for JSON transport
	return base64.StdEncoding.EncodeToString(sig), nil
}
