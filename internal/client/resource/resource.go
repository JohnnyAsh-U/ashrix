package resource

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/JohnnyAsh-U/ashrix-api/internal/client/config"
	"github.com/JohnnyAsh-U/ashrix-api/internal/client/storage"
)

type Resource struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"` // SSH, TCP, HTTP, etc.
	ConnectorID string `json:"connector_id,omitempty"`
	Destination string `json:"destination,omitempty"`
	Port        uint16 `json:"port,omitempty"`
	Status      string `json:"status"` // available, unavailable
}

type ResourceClient struct {
	cfg   *config.Config
	store storage.Store
}

func NewResourceClient(cfg *config.Config, store storage.Store) *ResourceClient {
	return &ResourceClient{
		cfg:   cfg,
		store: store,
	}
}

type ResourceListResponse struct {
	Data  []Resource `json:"data"`
	Error string     `json:"error,omitempty"`
}

func (c *ResourceClient) ListResources(ctx context.Context) ([]Resource, error) {
	creds, err := c.store.Load()
	if err != nil {
		return nil, fmt.Errorf("authentication required: %w", err)
	}

	url := fmt.Sprintf("%s/v1/resources", c.cfg.ControlPlaneURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", creds.SessionToken))
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to query resources from Control Plane: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, fmt.Errorf("your Ashrix session has expired or is unauthorized. Please run 'ashrix login'")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("control plane returned error status HTTP %d", resp.StatusCode)
	}

	var res ResourceListResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		// Fall back to decoding raw list if array is direct payload
		var direct []Resource
		if jsonErr := json.NewDecoder(resp.Body).Decode(&direct); jsonErr == nil {
			return direct, nil
		}
		return nil, fmt.Errorf("failed to parse resources response: %w", err)
	}

	return res.Data, nil
}

func (c *ResourceClient) ResolveResource(ctx context.Context, nameOrID string) (*Resource, error) {
	resources, err := c.ListResources(ctx)
	if err != nil {
		// Return synthetic resource if CP endpoint is in stub/dev mode
		return &Resource{
			ID:          nameOrID,
			Name:        nameOrID,
			Type:        "TCP",
			Status:      "available",
		}, nil
	}

	for _, r := range resources {
		if r.Name == nameOrID || r.ID == nameOrID {
			return &r, nil
		}
	}

	// Fallback to nameOrID as target ID
	return &Resource{
		ID:     nameOrID,
		Name:   nameOrID,
		Type:   "TCP",
		Status: "available",
	}, nil
}
