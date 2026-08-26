package router

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/policy"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/policy/store"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/registry"
	"github.com/JohnnyAsh-U/ashrix-api/pkg/flow"
	pb "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
)

var (
	ErrInvalidFlow       = errors.New("invalid flow")
	ErrUnauthorized      = errors.New("unauthorized")
	ErrPolicyDenied      = errors.New("access denied by policy")
	ErrAppNotFound       = errors.New("application not found")
	ErrConnectorNotFound = errors.New("connector not found")
	ErrConnectorOffline  = errors.New("connector offline")
)

type Router struct {
	registry *registry.Registry
	policy   *policy.PolicyEngine
	store    *store.BoltStore
}

func NewRouter(registry *registry.Registry, policy *policy.PolicyEngine, store *store.BoltStore) *Router {
	return &Router{
		registry: registry,
		policy:   policy,
		store:    store,
	}
}

func (r *Router) Route(ctx context.Context, req *pb.StreamFrame, evalPolicy bool) (flow.Stream, error) {

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
	} else if connEntry, ok := r.registry.GetBySubdomain(req.DestAppName); ok {
		destConnector = connEntry
		for _, app := range connEntry.Apps {
			if app.Subdomain == req.DestAppName {
				destApp = app
				break
			}
		}
	}

	if destApp == nil || destConnector == nil {
		return nil, ErrAppNotFound
	}

	if !destConnector.IsRoutable() {
		return nil, ErrConnectorOffline
	}

	authCtx := policy.AuthorizationContext{
		Principal: policy.Principal{
			UserID: req.SourceId, // App ID
			Name:   req.SourceEmail,
		},
		Resource: policy.Resource{
			AppID:  req.DestAppId,
			Path:   req.Path,
			Method: req.Method,
		},
		Time:     time.Now(),
		TenantID: req.TenantId,
	}

	//  Evaluate policy when it is app to app, user to app policy happens at middleware.
	if evalPolicy {
		decision := r.policy.Evaluate(authCtx)
		if decision.Denied() {
			return nil, fmt.Errorf("%w: %s", ErrPolicyDenied, decision.Reason)
		}
	}

	// 8. Open a stream to the destination connector.
	tunnel, ok := r.registry.GetTunnelSession(destConnector.ConnectorID)
	if !ok || tunnel == nil {
		return nil, ErrConnectorOffline
	}

	stream, err := tunnel.OpenStream(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to open stream on connector: %w", err)
	}

	return stream, nil
}
