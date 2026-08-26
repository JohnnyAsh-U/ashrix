package app

import (
	"time"

	"github.com/google/uuid"
)

type CreateAppRequest struct {
	Name        string     `json:"name" validate:"required"`
	Subdomain   string     `json:"subdomain" validate:"required,hostname_rfc1123"`
	Upstream    string     `json:"upstream" validate:"required,url"`
	Protocol    string     `json:"protocol" validate:"required,oneof=http https"`
	IsPublic    *bool      `json:"is_public" validate:"required"`
	ConnectorID *uuid.UUID `json:"connector_id,omitempty"`
	SockPass 	string 	   `json:"sock_pass"`
}

type UpdateAppRequest struct {
	Name        string     `json:"name" validate:"required"`
	Subdomain   string     `json:"subdomain" validate:"required,hostname_rfc1123"`
	Upstream    string     `json:"upstream" validate:"required,url"`
	Protocol    string     `json:"protocol" validate:"required,oneof=http https"`
	IsPublic    *bool      `json:"is_public" validate:"required"`
	ConnectorID *uuid.UUID `json:"connector_id,omitempty"`
	SockPass 	string 	   `json:"sock_pass"`
}

type AppResponse struct {
	ID          string     `json:"id"`
	OrgID       string     `json:"org_id"`
	ConnectorID *string    `json:"connector_id,omitempty"`
	Name        string     `json:"name"`
	Subdomain   string     `json:"subdomain"`
	Upstream    string     `json:"upstream"`
	Protocol    string     `json:"protocol"`
	IsPublic    bool       `json:"is_public"`
	SockPass 	string 	   `json:"sock_pass"`
	CreatedAt   time.Time  `json:"created_at"`
}
