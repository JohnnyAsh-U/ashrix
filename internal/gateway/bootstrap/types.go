package bootstrap

import "time"

type Request struct {
	Token string `json:"token"`
	CSR string `json:"csr_pem"`
	Timestamp time.Time `json:"timestamp"`
}

type RenewGatewayCertRequest struct {
	GatewayID string `json:"gateway_id" validate:"required"`
	Signature string `json:"signature" validate:"required"`
	CSR       string `json:"csr" validate:"required"`
	Timestamp time.Time `json:"timestamp"`
}

type EnrollResponse struct {
	GatewayID string `json:"gateway_id"`
	Certificate string    `json:"certificate"`
	TrustBundle string    `json:"trust_bundle"`
	ExpiresAt   time.Time `json:"expires_at"`
}



type ApiError struct {
	Code    string `json:"code"`
	Message string    `json:"message"`
	Status  int
	Details any `json:"details,omitempty"`
}


type APIResponse struct {
	Success bool      `json:"success"`
	Data    EnrollResponse       `json:"data,omitzero"`
	Error   ApiError `json:"error,omitzero"`
}
