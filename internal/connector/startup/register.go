package startup

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"time"

	gen "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type ApiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Status  int
	Details any `json:"details,omitempty"`
}

type APIRegisterResponse struct {
	Success bool                        `json:"success"`
	Data    gen.ConnectorEnrollResponse `json:"data,omitzero"`
	Error   ApiError                    `json:"error,omitzero"`
}

// Register runs the full new-connector registration flow.
// Generate ECDSA keypair
// Generate CSR
// Prompt User for token
// Send CSR + token to CP
// Return Response
func Register(ctx context.Context, cpURL string, log *zap.Logger, token string) (*gen.ConnectorEnrollResponse, *ecdsa.PrivateKey, error) {
	// Generate ECDSA p256
	log.Info("Generating ECDSA P-256 keypair")

	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to generate key %w", err)
	}

	//Generate CSR
	log.Info("Generating CSR")

	csrPem, err := generateCSR(privKey)
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to generate CSR %w", err)
	}

	//token
	if token == ""{
		return nil, nil, fmt.Errorf("Error: Token missing. Run 'ashrix-connector start --token=xxxxx' ")
	}

	//Send to CP
	log.Info("Registering Connector with Token on CP", zap.String("cp_url", cpURL))
	apiResp, err := sendRegisterRequest(ctx, token, cpURL, csrPem)

	if err != nil {
		return nil, nil, err
	}

	log.Info("Registration successful")
	return &apiResp.Data, privKey, nil

}

func sendRegisterRequest(ctx context.Context, token, cpUrl string, csrPEM []byte) (*APIRegisterResponse, error) {

	body, err := json.Marshal(&gen.ConnectorEnrollRequest{
		Token:     token,
		CsrPem:    string(csrPEM),
		Timestamp: timestamppb.Now(),
	})

	if err != nil {
		return nil, fmt.Errorf("Failed to encode bootstrap request %w", err)
	}

	httpCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(
		httpCtx,
		http.MethodPost,
		cpUrl+"/api/v1/internal/connectors/enroll",
		bytes.NewBuffer(body),
	)

	if err != nil {
		return nil, fmt.Errorf("Cannot reach CP: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

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

func generateCSR(privKey *ecdsa.PrivateKey) ([]byte, error) {
	template := &x509.CertificateRequest{
		Subject: pkix.Name{
			Organization:       []string{"Ashrix"},
			OrganizationalUnit: []string{"Connector"},
		},
	}

	csrDER, err := x509.CreateCertificateRequest(
		rand.Reader, template, privKey,
	)

	if err != nil {
		return nil, err
	}

	return pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE REQUEST",
		Bytes: csrDER,
	}), nil
}
