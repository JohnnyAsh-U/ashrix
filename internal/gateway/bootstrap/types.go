package bootstrap

import (
	"github.com/JohnnyAsh-U/ashrix-api/proto/gen"
)


type ApiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Status  int
	Details any `json:"details,omitempty"`
}

type APIResponse struct {
	Success bool                        `json:"success"`
	Data    proto.GatewayEnrollResponse `json:"data,omitzero"`
	Error   ApiError                    `json:"error,omitzero"`
}
