package gateway

import (
	"time"
)

type CreateGateway struct {
	Name      string `json:"name"`
	IPAddress string `json:"ip_address"`
	PublicURL string `json:"public_url"`
	LogToCP   bool   `json:"log_to_cp"`
}

type ReEnrollGatewayRequest struct {
	Name      string `json:"name"`
	IPAddress string `json:"ip_address"`
	PublicURL string `json:"public_url"`
	LogToCP   bool   `json:"log_to_cp"`
}


type RevokeGatewayRequest struct {
	RevokeReason string `json:"revoke_reason" validate:"required"`
}

// ==========================================================
// RESPONSE
// ==========================================================

type GatewayAppSummary struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Subdomain    string `json:"subdomain"`
	HealthStatus string `json:"health_status"`
}

type GatewayInstruction struct {
	DownloadCmd string `json:"download_cmd"`
	EnrollCmd   string `json:"enroll_cmd"`
	StartCmd    string `json:"start_cmd"`
}

type GatewayResponse struct {
	ID                    string              `json:"id"`
	Name                  string              `json:"name"`
	OrgID                 string              `json:"org_id"`
	Status                string              `json:"status"`
	Version               string              `json:"version"`
	DeploymentType        string              `json:"deployment_type"`
	PublicURL             string              `json:"public_url"`
	IPAddress             string              `json:"ip_address"`
	LastHeartBeat         *time.Time          `json:"last_heartbeat,omitempty"`
	Uptime int64 `json:"uptime,omitempty"`
	LogToCP               bool                `json:"log_to_cp"`
	EnrolledAt            *time.Time          `json:"enrolled_at,omitempty"`
	RevokedAt             *time.Time          `json:"revoked_at,omitempty"`
	NumberOfSessionActive int64               `json:"number_of_session_active"`
	PolicyVersion         int64               `json:"policy_version"`
	CurrentPolicyVersion  int64               `json:"current_policy_version"`
	NumberOfApps          int                 `json:"number_of_apps"`
	Apps                  []GatewayAppSummary `json:"apps"`
	Token                 string              `json:"token,omitempty"`

	DownloadURL string              `json:"download_url,omitempty"`
	DownloadCmd string              `json:"download_cmd,omitempty"`
	EnrollCmd   string              `json:"enroll_cmd,omitempty"`
	StartCmd    string              `json:"start_cmd,omitempty"`
	Instruction *GatewayInstruction `json:"instruction,omitempty"`

	CertificateIssuedAt  *time.Time `json:"certificate_issued_at,omitempty"`
	CertificateExpiresAt *time.Time `json:"certificate_expires_at,omitempty"`
	CreatedAt            time.Time  `json:"created_at"`
}
