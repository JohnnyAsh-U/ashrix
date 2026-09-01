package health

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	pb "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type AppHealthState struct {
	AppID        string
	HealthStatus string    // "healthy" | "unhealthy" | "unknown"
	LastSeen     time.Time
}

type Checker struct {
	mu         sync.RWMutex
	apps       map[string]*pb.ConnectorApps
	health     map[string]*AppHealthState
	httpClient *http.Client
	log        *slog.Logger
}

func NewChecker(log *slog.Logger, apps []*pb.ConnectorApps) *Checker {
	c := &Checker{
		apps:   make(map[string]*pb.ConnectorApps),
		health: make(map[string]*AppHealthState),
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
		log: log,
	}
	c.UpdateApps(apps)
	return c
}

func (c *Checker) UpdateApps(apps []*pb.ConnectorApps) {
	c.mu.Lock()
	defer c.mu.Unlock()

	newApps := make(map[string]*pb.ConnectorApps)
	for _, app := range apps {
		newApps[app.Id] = app
		if _, exists := c.health[app.Id]; !exists {
			c.health[app.Id] = &AppHealthState{
				AppID:        app.Id,
				HealthStatus: "unknown",
			}
		}
	}
	c.apps = newApps
}

func (c *Checker) Start(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	// Initial ping
	c.checkAllApps(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.checkAllApps(ctx)
		}
	}
}

func (c *Checker) checkAllApps(ctx context.Context) {
	c.mu.RLock()
	appsToPing := make([]*pb.ConnectorApps, 0, len(c.apps))
	for _, app := range c.apps {
		if app.CheckHealth {
			appsToPing = append(appsToPing, app)
		}
	}
	c.mu.RUnlock()

	for _, app := range appsToPing {
		c.pingApp(ctx, app)
	}
}

func (c *Checker) pingApp(ctx context.Context, app *pb.ConnectorApps) {
	endpoint := app.HealthEndpoint
	if endpoint == "" {
		endpoint = "/health"
	}
	if !strings.HasPrefix(endpoint, "/") {
		endpoint = "/" + endpoint
	}

	scheme := "http"
	if strings.ToLower(app.Protocol) == "https" {
		scheme = "https"
	}

	targetURL := fmt.Sprintf("%s://%s%s", scheme, app.Upstream, endpoint)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		c.updateState(app.Id, "unhealthy", time.Time{})
		return
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.log.Debug("app health ping failed", "app_id", app.Id, "url", targetURL, "error", err)
		c.updateState(app.Id, "unhealthy", time.Time{})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		c.updateState(app.Id, "healthy", time.Now())
	} else {
		c.log.Debug("app health ping non-200", "app_id", app.Id, "status_code", resp.StatusCode)
		c.updateState(app.Id, "unhealthy", time.Time{})
	}
}

func (c *Checker) updateState(appID, status string, lastSeen time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()

	state, ok := c.health[appID]
	if !ok {
		state = &AppHealthState{AppID: appID}
		c.health[appID] = state
	}
	state.HealthStatus = status
	if !lastSeen.IsZero() {
		state.LastSeen = lastSeen
	}
}

func (c *Checker) GetAppHealthStatuses() []*pb.AppHealthStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var result []*pb.AppHealthStatus
	for _, state := range c.health {
		item := &pb.AppHealthStatus{
			AppId:        state.AppID,
			HealthStatus: state.HealthStatus,
		}
		if !state.LastSeen.IsZero() {
			item.LastSeen = timestamppb.New(state.LastSeen)
		}
		result = append(result, item)
	}
	return result
}
