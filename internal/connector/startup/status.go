// internal/connector/startup/status.go
package startup

import (
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	gen "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
)

type APIStatusResponse struct {
	Success bool                        `json:"success"`
	Data    gen.ConnectorStatusResponse `json:"data,omitzero"`
	Error   ApiError                    `json:"error,omitzero"`
}

// FetchStatus calls the CP status endpoint and returns connector info.
// Uses private key signed payload — the connector cert authenticates the request.
func FetchStatus(
	ctx context.Context,
	cpURL string,
	key *ecdsa.PrivateKey,
	connectorID string,
	log *slog.Logger,
) (*gen.ConnectorStatusResponse, error) {

	log.Info("checking connector status with CP",
		"cp_url", cpURL,
	)

	//---Build Payload and signature --------------------
	nonce, err := generateNonce()
	if err != nil {
		return nil, err
	}
	ts := time.Now().Unix()
	statusPath := "/api/v1/internal/connectors/status"

	canonicalString := fmt.Sprintf("%s\n%s\n%s\n%s\n%d", http.MethodGet, statusPath, connectorID, nonce, ts)

	hash := sha256.Sum256([]byte(canonicalString))

	sig, err := ecdsa.SignASN1(
		rand.Reader,
		key,
		hash[:],
	)

	if err != nil {
		return nil, err
	}

	signature := base64.StdEncoding.EncodeToString(sig)

	// ── Make request ──────────────────────────────────────────────
	httpCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(
		httpCtx,
		http.MethodGet,
		cpURL+"/api/v1/internal/connectors/status",
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("build status request: %w", err)
	}

	req.Header.Set("X-Ashrix-Connector-ID", connectorID)
	req.Header.Set("X-Ashrix-Timestamp", strconv.FormatInt(ts, 10))
	req.Header.Set("X-Ashrix-Signature", signature)
	req.Header.Set("X-Ashrix-Nonce", nonce)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("status request failed: %w", err)
	}
	defer resp.Body.Close()

	var apiResponse APIStatusResponse

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

	log.Info("connector status OK",
		"connector_id", apiResponse.Data.ConnectorId,
		"gateway_id", apiResponse.Data.GatewayId,
		"gateway_url", apiResponse.Data.GatewayUrl,
		"apps", len(apiResponse.Data.Apps),
	)

	for _, app := range apiResponse.Data.Apps {
		log.Info("registered app",
			"name", app.Name,
			"subdomain", app.Subdomain,
			"upstream", app.Upstream,
			"protocol", app.Protocol,
			"is_public", app.IsPublic,
		)
	}

	return &apiResponse.Data, nil
}

func generateNonce() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("Error generating nonce")
	}

	return hex.EncodeToString(b), nil
}
