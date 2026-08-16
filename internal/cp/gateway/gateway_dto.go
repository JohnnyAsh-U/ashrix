package gateway

import (
	"time"
)

type CreateGateway struct {
	Name  string    `json:"name"`
	IPAddress string `json:"ip_address"`
	PublicURL string `json:"public_url"`
}

type ReEnrollGatewayRequest struct {
	Name  string    `json:"name"`
	IPAddress string `json:"ip_address"`
	PublicURL string `json:"public_url"`
}

type RevokeGatewayCertRequest struct {
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
