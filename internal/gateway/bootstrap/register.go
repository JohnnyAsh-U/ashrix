package bootstrap

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	gen "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Client struct {
	CPURL string
	HTTP  *http.Client
}

func NewClient(cpURL string) *Client {
	return &Client{
		CPURL: cpURL,
		HTTP: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *Client) Bootstrap(token string, csrPEM string) (*APIRegisterResponse, error) {
	reqBody := gen.GatewayEnrollRequest{
		Token:     token,
		CsrPem:       csrPEM,
		Timestamp: timestamppb.Now(),
	}

	body, err := json.Marshal(&reqBody)

	if err != nil {
		return nil, fmt.Errorf("Failed to encode bootstrap request %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, c.CPURL+"/api/v1/internal/gateways/enroll", bytes.NewBuffer(body))

	if err != nil {
		return nil, fmt.Errorf("Cannot reach CP: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTP.Do(req)

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
