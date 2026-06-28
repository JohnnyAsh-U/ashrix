package startup

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/url"
	"time"

	gen "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Register runs the full new-connector registration flow.
// Generate ECDSA keypair
// Generate CSR
// Build possession proof with current old key
// Send CSR + token to CP
// Return Response
func Renew(
	ctx context.Context,
	cpURL, connectorID string,
	log *zap.Logger,
	currentKey *ecdsa.PrivateKey,
) (*gen.ConnectorEnrollResponse, *ecdsa.PrivateKey, error) {

	log.Info("renewing connector certificate",
		zap.String("connector_id", connectorID),
	)
	// ── 1. Generate new keypair ───────────────────────────────────

	newKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("generate new keypair: %w", err)
	}

	// ── 2. Generate CSR with new key ──────────────────────────────
	csrPEM, err := generateRenewCSR(newKey, connectorID)
	if err != nil {
		return nil, nil, fmt.Errorf("generate CSR: %w", err)
	}
	// ── 3. Build possession proof ─────────────────────────────────
	// Proves to CP that we hold the current private key.
	// CP verifies against the public key in the current cert.
	timestamp := time.Now().UTC().Unix()
	proof, err := buildPossessionProof(currentKey, csrPEM, connectorID, timestamp)
	if err != nil {
		return nil, nil, fmt.Errorf("build possession proof: %w", err)
	}

	// ── 4. Send renewal request ───────────────────────────────────
	apiResp, err := sendRenewRequest(ctx, cpURL, connectorID, csrPEM, proof, timestamp)
	if err != nil {
		return nil, nil, err
	}

	log.Info("certificate renewed successfully")

	return &apiResp.Data, newKey, nil

}

func sendRenewRequest(ctx context.Context, cpUrl, connectorID string, csrPEM []byte, proof string,
	timestamp int64) (*APIRegisterResponse, error) {

	body, err := json.Marshal(&gen.ConnectorRenewCertRequest{
		ConnectorId: connectorID,
		Signature:   proof,
		CsrPem:      string(csrPEM),
		Timestamp:   timestamppb.New(time.Unix(timestamp, 0).UTC()),
	})

	if err != nil {
		return nil, fmt.Errorf("Failed to encode bootstrap request %w", err)
	}

	httpCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(
		httpCtx,
		http.MethodPost,
		cpUrl+"/api/v1/internal/connectors/renew",
		bytes.NewBuffer(body),
	)

	if err != nil {
		return nil, fmt.Errorf("Cannot reach CP: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Ashrix-Connector-ID", connectorID)

	resp, err := http.DefaultClient.Do(req)

	if err != nil {
		return nil, fmt.Errorf("Cannot react CP %w", err)
	}

	defer resp.Body.Close()

	var apiResponse APIRegisterResponse

	if err := json.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
		return nil, fmt.Errorf("Failed to Parse CP response: %s", err)
	}

	if resp.StatusCode >= http.StatusBadRequest {
		switch apiResponse.Error.Status {
		case http.StatusInternalServerError:
			return nil, fmt.Errorf("An Error Occurred")
		case http.StatusBadRequest:
			return nil, fmt.Errorf("Error parsing CSR")
		case http.StatusNotFound:
			return nil, fmt.Errorf("Unfound Token or Invalid Token ")
		case http.StatusUnauthorized:
			return nil, fmt.Errorf("CSR rejected by CP")
		default:
			return nil, fmt.Errorf("An unknown Error Occurred")
		}
	}

	return &apiResponse, nil
}

func generateRenewCSR(privKey *ecdsa.PrivateKey, connectorID string) ([]byte, error) {
	connectorURI, _ := url.Parse("ashrix://connector/" + connectorID)

	template := &x509.CertificateRequest{
		Subject: pkix.Name{
			CommonName:         connectorID,
			Organization:       []string{"Ashrix"},
			OrganizationalUnit: []string{"Connector"},
		},
		URIs: []*url.URL{connectorURI},
	}

	csrDER, err := x509.CreateCertificateRequest(rand.Reader, template, privKey)
	if err != nil {
		return nil, err
	}

	return pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE REQUEST",
		Bytes: csrDER,
	}), nil
}


// buildPossessionProof signs SHA256(csr || connectorID || timestamp_bytes)
// with the current private key.
func buildPossessionProof(
	currentKey *ecdsa.PrivateKey,
	csrPEM []byte,
	connectorID string,
	timestamp int64,
) (string, error) {

	h := sha256.New()
	h.Write(csrPEM)
	h.Write([]byte(connectorID))

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
