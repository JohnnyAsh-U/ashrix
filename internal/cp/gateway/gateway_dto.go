package gateway

import (
	"github.com/google/uuid"
	"time"
)

type CreateGateway struct {
	OrgID uuid.UUID `json:"org_id"`
	Name  string    `json:"name"`
}

type ReEnrollGatewayRequest struct {
	OrgID uuid.UUID `json:"org_id"`
	Name  string    `json:"name"`
}

type RevokeGatewayCertRequest struct {
	ComponentID   uuid.UUID `json:"component_id"`
	ComponentType string    `json:"component_type"`
	RevokeReason  string    `json:"revoke_reason"`
}

type RevokeGatewayRequest struct {
	RevokeReason string `json:"revoke_reason" validate:"required"`
}

// ==========================================================
// RESPONSE
// ==========================================================

type GatewayResponse struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	OrgID         string    `json:"org_id"`
	Version       string    `json:"version"`
	LastHeartBeat time.Time `json:"last_heartbeat"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
	EnrolledAt    time.Time `json:"enrolled_at"`
	RevokedAt     time.Time `json:"revoked_at"`
}
