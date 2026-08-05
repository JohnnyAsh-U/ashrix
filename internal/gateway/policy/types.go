package policy

import (
	"fmt"
	"net"
	"time"
)

// =============================================================================
// Policy Types & Contexts
// =============================================================================

type PolicyResponse struct {
	PolicyID    string           `json:"policy_id"`
	TenantID    string           `json:"tenant_id"`
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Effect      Effect           `json:"effect"`
	Priority    int              `json:"priority"`
	Subject     SubjectSelector  `json:"subject"`
	Resource    ResourceSelector `json:"resource"`
	Conditions  PolicyConditions `json:"conditions"`
	Enabled     bool             `json:"enabled"`
	Version     int64            `json:"version"`
	CreatedBy   string           `json:"created_by"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
}

type Effect string

const (
	EffectAllow Effect = "ALLOW"
	EffectDeny  Effect = "DENY"
)

type SubjectSelector struct {
	Users  []string `json:"users"`
	Groups []string `json:"groups"`
}

type ResourceSelector struct {
	Apps    []string `json:"apps"`
	Paths   []string `json:"paths"`
	Methods []string `json:"methods"`
}

type PolicyConditions struct {
	MFA     *MFACondition     `json:"mfa,omitempty"`
	Device  *DeviceCondition  `json:"device,omitempty"`
	Network *NetworkCondition `json:"network,omitempty"`
	Time    *TimeCondition    `json:"time,omitempty"`
}

type MFACondition struct {
	Required bool   `json:"required"`
	MinLevel string `json:"min_level,omitempty"`
}

type DeviceCondition struct {
	Postures []string `json:"postures"`
}

type NetworkCondition struct {
	AllowedCountries []string `json:"allowed_countries,omitempty"`
	BlockedCountries []string `json:"blocked_countries,omitempty"`
	AllowedCIDRs     []string `json:"allowed_cidrs,omitempty"`
	BlockedCIDRs     []string `json:"blocked_cidrs,omitempty"`
	BlockTor         bool     `json:"block_tor,omitempty"`
}

type TimeCondition struct {
	ScheduleName string `json:"schedule_name"`
}

type AuthorizationContext struct {
	Principal Principal
	Device    DeviceContext
	Network   NetworkContext
	Resource  Resource
	Time      time.Time
	GatewayID string
	TenantID  string
	RequestID string
}

type Principal struct {
	UserID   string   `json:"user_id"`
	Name     string   `json:"name"`
	Email    string   `json:"email"`
	Groups   []string `json:"groups"`
	MFALevel string   `json:"mfa_level"`
}

type DeviceContext struct {
	Posture string `json:"posture"`
	ID      string `json:"id"`
}

type NetworkContext struct {
	SourceIP  net.IP `json:"source_ip"`
	Country   string `json:"country"`
	IsTorExit bool   `json:"is_tor_exit"`
}

type Resource struct {
	AppID  string `json:"app_id"`
	Path   string `json:"path"`
	Method string `json:"method"`
}

type Decision struct {
	Effect        Effect    `json:"effect"`
	PolicyID      string    `json:"policy_id"`
	Reason        string    `json:"reason"`
	EvaluatedAt   time.Time `json:"evaluated_at"`
	UserID        string    `json:"user_id"`
	AppID         string    `json:"app_id"`
	GatewayID     string    `json:"gateway_id"`
	TenantID      string    `json:"tenant_id"`
	PolicyVersion int64     `json:"policy_version"`
}

func (d Decision) Allowed() bool { return d.Effect == EffectAllow }
func (d Decision) Denied() bool  { return d.Effect == EffectDeny }

func (d Decision) String() string {
	if d.PolicyID == "" {
		return fmt.Sprintf("DENY(default) reason=%s", d.Reason)
	}
	return fmt.Sprintf("%s policy=%s reason=%s", d.Effect, d.PolicyID, d.Reason)
}