package bootstrap

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	gen "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (c *Client) RenewCert(proof string, csrPEM string, gatewayID string) (*APIRenewResponse, error) {

	body, err := json.Marshal(gen.GatewayRenewCertRequest{
		Signature: proof,
		CsrPem:    csrPEM,
		GatewayId: gatewayID,
		Timestamp: timestamppb.Now(),
	})

	if err != nil {
		return nil, fmt.Errorf("Failed to encode bootstrap request %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

	defer cancel()

	req, err := http.NewRequestWithContext(ctx,
		http.MethodPost,
		c.CPURL+"/api/v1/internal/gateways/renew",
		bytes.NewBuffer(body),
	)

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTP.Do(req)

	if err != nil {
		return nil, fmt.Errorf("Cannot reach CP: %w", err)
	}

	defer resp.Body.Close()

	var apiResponse APIRenewResponse

	if err := json.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
		return nil, fmt.Errorf("Failed to Parse CP response: %s", err)
	}

	// if resp.StatusCode >= http.StatusBadRequest {
	// 	switch apiResponse.Error.Status {
	// 	case http.StatusInternalServerError:
	// 		return &apiResponse, nil
	// 	case http.StatusBadRequest:
	// 		return nil, fmt.Errorf("400: Error parsing CSR")
	// 	case http.StatusNotFound:
	// 		return nil, fmt.Errorf("404: Gateway Not Found Or Current Cert Not Found")
	// 	case http.StatusUnauthorized:
	// 		return nil, fmt.Errorf("403: Rejected By CP")
	// 	default:
	// 		return nil, fmt.Errorf("500: An unknown Error Occurred")
	// 	}
	// }

	return &apiResponse, nil
}
