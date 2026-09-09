package connector

import (
	"github.com/google/uuid"
	"time"
)

type CreateConnector struct {
	Name               string     `json:"name"`
	GatewayID          uuid.UUID  `json:"gateway_id"`
	SecondaryGatewayID *uuid.UUID `json:"secondary_gateway_id,omitempty"`
	OpenSock           bool       `json:"open_sock"`
}

type ReEnrollConnectorRequest struct {
	Name               string     `json:"name"`
	GatewayID          uuid.UUID  `json:"gateway_id"`
	SecondaryGatewayID *uuid.UUID `json:"secondary_gateway_id,omitempty"`
	OpenSock           bool       `json:"open_sock"`
}

type RevokeConnectorCertRequest struct {
	RevokeReason string `json:"revoke_reason"`
}

type RevokeConnectorRequest struct {
	RevokeReason string `json:"revoke_reason" validate:"required"`
}

// ==========================================================
// RESPONSE
// ==========================================================

type ConnectorAppSummaryResponse struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Subdomain    string `json:"subdomain"`
	HealthStatus string `json:"health_status"`
}

type ConnectorInstruction struct {
	DownloadCmd string `json:"download_cmd"`
	StartCmd    string `json:"start_cmd"`
}

type ConnectorResponse struct {
	ID                   string                        `json:"id"`
	Name                 string                        `json:"name"`
	OrgID                string                        `json:"org_id"`
	GatewayID            string                        `json:"gateway_id"`
	GatewayName          string                        `json:"gateway_name"`
	SecondaryGatewayID   *string                       `json:"secondary_gateway_id,omitempty"`
	SecondaryGatewayName *string                       `json:"secondary_gateway_name,omitempty"`
	Version              string                        `json:"version"`
	LastHeartBeat        *time.Time                    `json:"last_seen,omitempty"`
	Token                string                        `json:"token,omitempty"`
	Status               string                        `json:"status"`
	OpenSock             bool                          `json:"open_sock"`
	ActiveStream         int32                         `json:"active_stream"`
	Apps                 []ConnectorAppSummaryResponse `json:"apps"`
	CreatedAt            time.Time                     `json:"created_at"`
	EnrolledAt           *time.Time                    `json:"enrolled_at,omitempty"`
	RevokedAt            *time.Time                    `json:"revoked_at,omitempty"`

	Instruction *ConnectorInstruction `json:"instruction,omitempty"`
}
