package graph


import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"golang.org/x/oauth2/clientcredentials"
)

type EntraClient struct {
	httpClient *http.Client
}

func NewEntraClient(tenantID, clientID, clientSecret string) *EntraClient {
	cfg := clientcredentials.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		TokenURL:     fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/token", tenantID),
		Scopes:       []string{"https://graph.microsoft.com/.default"},
	}
	return &EntraClient{httpClient: cfg.Client(context.Background())}
}

type graphGroupResponse struct {
	DisplayName string `json:"displayName"`
}

func (c *EntraClient) ResolveGroupNames(ctx context.Context, groupGUIDs []string) ([]string, error) {
	var names []string

	for _, guid := range groupGUIDs {
		reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		url := fmt.Sprintf("https://graph.microsoft.com/v1.0/groups/%s?$select=displayName", guid)

		req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
		if err != nil {
			cancel()
			return nil, err
		}

		resp, err := c.httpClient.Do(req)
		cancel()
		if err != nil {
			return nil, fmt.Errorf("graph lookup failed for group %s: %w", guid, err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("graph returned %d for group %s", resp.StatusCode, guid)
		}

		var g graphGroupResponse
		if err := json.NewDecoder(resp.Body).Decode(&g); err != nil {
			return nil, fmt.Errorf("decode graph response: %w", err)
		}
		names = append(names, g.DisplayName)
	}

	return names, nil
}