package dto

import (
	"encoding/json"
	"net/http"
)

type APIResponse struct {
	Success bool      `json:"success"`
	Data    any       `json:"data,omitempty"`
	Error   *AppError `json:"error,omitempty"`
}

//SendSuccess handle

func SendSuccess(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(APIResponse{
		Success: true,
		Data:    data,
	})
}

func SendError(w http.ResponseWriter, appErr *AppError) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(appErr.Status)

	json.NewEncoder(w).Encode(APIResponse{
		Success: false,
		Error:   appErr,
	})
}
