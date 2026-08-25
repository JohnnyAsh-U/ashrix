package router

import (
	"context"
	"errors"
	"fmt"
	// "time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/policy"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/policy/store"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/registry"
	"github.com/JohnnyAsh-U/ashrix-api/pkg/flow"
	pb "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	"golang.org/x/crypto/bcrypt"
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

func (r *Router) Route(ctx context.Context, req flow.OpenRequest) (flow.Stream, error) {
	// 1. Validate request.
	if req.Destination.AppID == "" {
		return nil, ErrInvalidFlow
	}

	// 2. Resolve destination application and connector.
	var destApp *pb.ConnectorApps
	var destConnector *registry.ConnectorEntry

	if connEntry, ok := r.registry.GetByAppID(req.Destination.AppID); ok {
		destConnector = connEntry
		for _, app := range connEntry.Apps {
			if app.Id == req.Destination.AppID {
				destApp = app
				break
			}
		}
	} else if connEntry, ok := r.registry.GetBySubdomain(req.Destination.AppID); ok {
		destConnector = connEntry
		for _, app := range connEntry.Apps {
			if app.Subdomain == req.Destination.AppID {
				destApp = app
				break
			}
		}
	}

	if destApp == nil || destConnector == nil {
		return nil, ErrAppNotFound
	}

	// Canonicalize App ID
	req.Destination.AppID = destApp.Id

	if !destConnector.IsRoutable() {
		return nil, ErrConnectorOffline
	}

	// 5. Build complete policy context.
	// var principal policy.Principal
	// if req.FlowType == flow.FlowUserToApp {
	// 	principal = policy.Principal{
	// 		UserID: req.Source.PrincipalID,
	// 		Email:  req.Source.PrincipalID,
	// 	}
	// } else {
	// 	principal = policy.Principal{
	// 		UserID: req.Source.PrincipalID, // Connector ID
	// 		Groups: []string{"m2m"},
	// 	}
	// }

	// authCtx := policy.AuthorizationContext{
	// 	Principal: principal,
	// 	Resource: policy.Resource{
	// 		AppID:  req.Destination.AppID,
	// 		Path:   req.HTTPPath,
	// 		Method: req.HTTPMethod,
	// 	},
	// 	Time:     time.Now(),
	// 	TenantID: destConnector.TenantID,
	// }

	// // 6. Evaluate policy.
	// decision := r.policy.Evaluate(authCtx)
	// if decision.Denied() {
	// 	return nil, fmt.Errorf("%w: %s", ErrPolicyDenied, decision.Reason)
	// }

	// 7. For SOCKS flows (App to App), validate SOCKS credentials.
	if req.FlowType == flow.FlowAppToApp {
		if req.SocksUsername == "" || req.SocksPassword == "" {
			return nil, fmt.Errorf("%w: missing socks credentials", ErrUnauthorized)
		}

		// Retrieve the credential for the source connector
		cred, err := r.store.GetSOCKS5CredentialByConnectorID(ctx, req.Source.PrincipalID)
		if err != nil {
			return nil, fmt.Errorf("%w: socks credential not found for connector: %v", ErrUnauthorized, err)
		}

		// Important invariant: SOCKS credential must belong to the source connector
		if cred.Username != req.SocksUsername {
			return nil, fmt.Errorf("%w: SOCKS username mismatch", ErrUnauthorized)
		}

		// Compare hash
		if err := bcrypt.CompareHashAndPassword([]byte(cred.PasswordHash), []byte(req.SocksPassword)); err != nil {
			return nil, fmt.Errorf("%w: invalid password", ErrUnauthorized)
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
