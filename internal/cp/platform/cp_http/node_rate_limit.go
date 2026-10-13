package cp_http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/connector"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/gateway"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/middleware"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/utils"
	gen "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

func gatewayNodeIDResolver(repo gateway.Repository, redisClient *redis.Client) middleware.NodeIDResolver {
	return func(r *http.Request, body []byte) string {
		switch {
		case strings.HasSuffix(r.URL.Path, "/enroll"):
			var payload gen.GatewayEnrollRequest
			if json.Unmarshal(body, &payload) != nil || payload.Token == "" {
				return ""
			}
			return resolveEnrolledNodeID(r.Context(), redisClient, "gateway", payload.Token, func() (string, error) {
				node, err := repo.GetGatewayByTokenHash(r.Context(), utils.HashToken(payload.Token))
				if err != nil {
					return "", err
				}
				return node.ID.String(), nil
			})
		case strings.HasSuffix(r.URL.Path, "/renew"):
			var payload gen.GatewayRenewCertRequest
			if json.Unmarshal(body, &payload) == nil {
				return canonicalNodeID(payload.GatewayId)
			}
		case strings.HasSuffix(r.URL.Path, "/status"):
			return canonicalNodeID(r.Header.Get("X-Ashrix-Gateway-ID"))
		}
		return ""
	}
}

func connectorNodeIDResolver(repo connector.Repository, redisClient *redis.Client) middleware.NodeIDResolver {
	return func(r *http.Request, body []byte) string {
		switch {
		case strings.HasSuffix(r.URL.Path, "/enroll"):
			var payload gen.ConnectorEnrollRequest
			if json.Unmarshal(body, &payload) != nil || payload.Token == "" {
				return ""
			}
			return resolveEnrolledNodeID(r.Context(), redisClient, "connector", payload.Token, func() (string, error) {
				node, err := repo.GetConnectorByTokenHash(r.Context(), utils.HashToken(payload.Token))
				if err != nil {
					return "", err
				}
				return node.ID.String(), nil
			})
		case strings.HasSuffix(r.URL.Path, "/renew"):
			var payload gen.ConnectorRenewCertRequest
			if json.Unmarshal(body, &payload) == nil {
				return canonicalNodeID(payload.ConnectorId)
			}
		case strings.HasSuffix(r.URL.Path, "/status"):
			return canonicalNodeID(r.Header.Get("X-Ashrix-Connector-ID"))
		}
		return ""
	}
}

func canonicalNodeID(nodeID string) string {
	id, err := uuid.Parse(nodeID)
	if err != nil {
		return ""
	}
	return id.String()
}

func resolveEnrolledNodeID(ctx context.Context, redisClient *redis.Client, nodeType, token string, lookup func() (string, error)) string {
	tokenHash := utils.HashToken(token)
	cacheKey := fmt.Sprintf("cp:enrollment-node:%s:%s", nodeType, tokenHash)
	if nodeID, err := redisClient.Get(ctx, cacheKey).Result(); err == nil && nodeID != "" {
		_ = redisClient.Expire(ctx, cacheKey, time.Hour).Err()
		return nodeID
	}

	nodeID, err := lookup()
	if err != nil || nodeID == "" {
		return ""
	}
	_ = redisClient.Set(ctx, cacheKey, nodeID, time.Hour).Err()
	return nodeID
}
