package router

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/registry"
	"github.com/JohnnyAsh-U/ashrix-api/internal/flow"
	pb "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
)

var (
	ErrInvalidFlow       = errors.New("invalid flow")
	ErrUnauthorized      = errors.New("unauthorized")
	ErrAppNotFound       = errors.New("application not found")
	ErrConnectorNotFound = errors.New("connector not found")
	ErrConnectorOffline  = errors.New("connector offline")
)

type Router struct {
	registry *registry.Registry
}

func NewRouter(registry *registry.Registry) *Router {
	return &Router{
		registry: registry,
	}
}

func (r *Router) Route(ctx context.Context, req *pb.StreamFrame) (flow.Stream, error) {

	// 2. Resolve destination application and connector.
	var destApp *pb.ConnectorApps
	var destConnector *registry.ConnectorEntry

	if connEntry, ok := r.registry.GetByAppID(req.DestAppId); ok {
		destConnector = connEntry
		for _, app := range connEntry.Apps {
			if app.Id == req.DestAppId {
				destApp = app
				break
			}
		}
	} else {
		var subDomain string
		parts := strings.Split(req.DestAppName, ".")
		if len(parts) > 2 {
			subDomain = strings.Join(parts[:len(parts)-2], ".")
		}

		if connEntry, ok := r.registry.GetBySubdomain(subDomain); ok {
			destConnector = connEntry
			for _, app := range connEntry.Apps {
				if app.Name == subDomain {
					destApp = app
					break
				}
			}
		}
	}

	if destApp == nil || destConnector == nil {
		return nil, ErrAppNotFound
	}

	if !destConnector.IsRoutable() {
		return nil, ErrConnectorOffline
	}

	// 8. Open a stream to the destination connector.
	tunnel, ok := r.registry.GetTunnelSession(destConnector.ConnectorID)
	if !ok || tunnel == nil {
		return nil, ErrConnectorOffline
	}

	req.DestAppId = destApp.Id

	stream, err := tunnel.OpenStream(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to open stream on connector: %w", err)
	}

	return stream, nil
}
