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

type EnrollGatewayRequest struct {
	Token string `json:"token" validate:"required"`
	CSR   string `json:"csr_pem" validate:"required"`
	Timestamp time.Time `json:"timestamp" validate:"required"`
}

type RenewGatewayCertRequest struct {
	GatewayID string `json:"gateway_id" validate:"required"`
	Signature string `json:"signature" validate:"required"`
	CSR       string `json:"csr" validate:"required"`
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
type EnrollResponse struct {
	GatewayID string `json:"gateway_id"`
	Certificate string    `json:"certificate"`
	TrustBundle string    `json:"trust_bundle"`
	ExpiresAt   time.Time `json:"expires_at"`
}

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
