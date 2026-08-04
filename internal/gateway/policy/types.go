package policy

import (
	"fmt"
	"net"
	"time"

	proto "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
)

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

// --- Authorization Context ---

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
	Name string `json:"name"`
	Email    string   `json:"email"`
	Groups   []string `json:"groups"`
	MFALevel string `json:"mfa_level"`

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

// --- Decision ---

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


// --- Compiled Policy ---

type CompiledPolicy struct {
	Proto   *proto.PolicyRule
	Effect  Effect
	PolicyID string
	Priority int
	Version  int64

	AppIDs         map[string]struct{}
	Groups         map[string]struct{}
	Users          map[string]struct{}
	Methods        map[string]struct{}
	Countries      map[string]struct{}
	BlockCountries map[string]struct{}
	AllowedCIDRs   []*net.IPNet
	BlockedCIDRs   []*net.IPNet

	BlockTor       bool
	RequireMFA     bool
	MinMFALevel    string
	RequirePosture []string
	HasTimeCond    bool
	ScheduleName   string
}

func (cp *CompiledPolicy) matchesSubject(p Principal) bool {
	if len(cp.Groups) > 0 {
		matched := false
		for _, g := range p.Groups {
			if _, ok := cp.Groups[g]; ok {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	if len(cp.Users) > 0 {
		if _, ok := cp.Users[p.UserID]; !ok {
			return false
		}
	}
	return true
}

func (cp *CompiledPolicy) matchesResource(r Resource) bool {
	if len(cp.AppIDs) > 0 {
		if _, ok := cp.AppIDs[r.AppID]; !ok {
			if _, ok := cp.AppIDs["*"]; !ok {
				return false
			}
		}
	}
	if len(cp.Methods) > 0 {
		if _, ok := cp.Methods[r.Method]; !ok {
			if _, ok := cp.Methods["*"]; !ok {
				return false
			}
		}
	}
	return true
}

func (cp *CompiledPolicy) matchesConditions(ctx AuthorizationContext) (bool, string) {
	if cp.RequireMFA {
		if ctx.Principal.MFALevel == "" {
			return false, "mfa_required"
		}
		if cp.MinMFALevel != "" {
			if !mfaLevelSufficient(ctx.Principal.MFALevel, cp.MinMFALevel) {
				return false, fmt.Sprintf("mfa_level_insufficient: got %s, need %s", ctx.Principal.MFALevel, cp.MinMFALevel)
			}
		}
	}

	if len(cp.RequirePosture) > 0 {
		matched := false
		for _, p := range cp.RequirePosture {
			if ctx.Device.Posture == p {
				matched = true
				break
			}
		}
		if !matched {
			return false, fmt.Sprintf("device_posture_failed: got %s, need %v", ctx.Device.Posture, cp.RequirePosture)
		}
	}

	if cp.BlockTor && ctx.Network.IsTorExit {
		return false, "tor_exit_blocked"
	}

	if len(cp.Countries) > 0 {
		if _, ok := cp.Countries[ctx.Network.Country]; !ok {
			return false, fmt.Sprintf("country_not_allowed: %s", ctx.Network.Country)
		}
	}

	if len(cp.BlockCountries) > 0 {
		if _, ok := cp.BlockCountries[ctx.Network.Country]; ok {
			return false, fmt.Sprintf("country_blocked: %s", ctx.Network.Country)
		}
	}

	if len(cp.AllowedCIDRs) > 0 {
		allowed := false
		for _, cidr := range cp.AllowedCIDRs {
			if cidr.Contains(ctx.Network.SourceIP) {
				allowed = true
				break
			}
		}
		if !allowed {
			return false, fmt.Sprintf("cidr_not_allowed: %s", ctx.Network.SourceIP)
		}
	}

	if len(cp.BlockedCIDRs) > 0 {
		for _, cidr := range cp.BlockedCIDRs {
			if cidr.Contains(ctx.Network.SourceIP) {
				return false, fmt.Sprintf("cidr_blocked: %s", ctx.Network.SourceIP)
			}
		}
	}

	return true, ""
}

func mfaLevelSufficient(have, need string) bool {
	levels := map[string]int{"": 0, "totp": 1, "push": 2, "webauthn": 3}
	return levels[have] >= levels[need]
}
