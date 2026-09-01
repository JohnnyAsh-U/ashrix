package app

import (
	"time"

	"github.com/google/uuid"
)

type CreateAppRequest struct {
	Name           string     `json:"name" validate:"required"`
	Subdomain      string     `json:"subdomain" validate:"required,hostname_rfc1123"`
	Upstream       string     `json:"upstream" validate:"required"`
	Protocol       string     `json:"protocol" validate:"required,oneof=http https tcp ssh"`
	IsPublic       *bool      `json:"is_public" validate:"required"`
	ConnectorID    *uuid.UUID `json:"connector_id,omitempty"`
	SockPass       string     `json:"sock_pass"`
	CheckHealth    *bool      `json:"check_health,omitempty"`
	CheckInterval  *int32     `json:"check_interval,omitempty"`
	HealthEndpoint string     `json:"health_endpoint,omitempty"`
}

type UpdateAppRequest struct {
	Name           string     `json:"name" validate:"required"`
	Subdomain      string     `json:"subdomain" validate:"required,hostname_rfc1123"`
	Upstream       string     `json:"upstream" validate:"required"`
	Protocol       string     `json:"protocol" validate:"required,oneof=http https tcp ssh"`
	IsPublic       *bool      `json:"is_public" validate:"required"`
	ConnectorID    *uuid.UUID `json:"connector_id,omitempty"`
	SockPass       string     `json:"sock_pass"`
	CheckHealth    *bool      `json:"check_health,omitempty"`
	CheckInterval  *int32     `json:"check_interval,omitempty"`
	HealthEndpoint string     `json:"health_endpoint,omitempty"`
}

type AppPolicySummary struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Effect   string `json:"effect"`
	Priority int32  `json:"priority"`
}

type AppResponse struct {
	ID                   string             `json:"id"`
	OrgID                string             `json:"org_id"`
	ConnectorID          *string            `json:"connector_id,omitempty"`
	ConnectorName        string             `json:"connector_name,omitempty"`
	GatewayName          string             `json:"gateway_name,omitempty"`
	Name                 string             `json:"name"`
	Subdomain            string             `json:"subdomain"`
	Upstream             string             `json:"upstream"`
	Protocol             string             `json:"protocol"`
	IsPublic             bool               `json:"is_public"`
	SockPass             string             `json:"sock_pass"`
	CheckHealth          bool               `json:"check_health"`
	CheckInterval        int32              `json:"check_interval"`
	HealthEndpoint       string             `json:"health_endpoint"`
	HealthStatus         string             `json:"health_status"`
	LastSeen             *time.Time         `json:"last_seen,omitempty"`
	NumberOfTrafficToday int64              `json:"number_of_traffic_today"`
	Policies             []AppPolicySummary `json:"policies"`
	NumberOfUsers        int64              `json:"number_of_users"`
	NumberOfGroups       int64              `json:"number_of_groups"`
	CreatedAt            time.Time          `json:"created_at"`
}
