package policy

import "time"

type Effect string

const (
	EffectAllow Effect = "ALLOW"
	EffectDeny  Effect = "DENY"
)

type Subject struct {
	Type  string `json:"type"`  // "group" or "user"
	Value string `json:"value"` // "engineering", "u-123"
}

type Resource struct {
	Type  string `json:"type"`  // "appID", "path", "method"
	Value string `json:"value"` // "id-jenkins-prod", "/*", "GET"
}

type Conditions struct {
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

// type CreatePolicyRequest struct {
// 	Name        string `json:"name" validate:"required,max=255"`
// 	Description string `json:"description" validate:"max=1000"`
// 	Effect      Effect `json:"effect" validate:"required,oneof=ALLOW DENY"`
// 	Priority    int    `json:"priority" validate:"min=0,max=1000"`

// 	SubjectUsers  []Subject `json:"subject_users"`
// 	SubjectGroups []Resource `json:"subject_groups"`

// 	ResourceApps    []string `json:"resource_apps" validate:"required,min=1"`
// 	ResourcePaths   []string `json:"resource"`
// 	ResourceMethods Conditions `json:"conditions"`

// 	Conditions PolicyConditions `json:"conditions"`
// }

// type UpdatePolicyRequest struct {
// 	Name            *string           `json:"name,omitempty"`
// 	Description     *string           `json:"description,omitempty"`
// 	Effect          *Effect           `json:"effect,omitempty"`
// 	Priority        *int              `json:"priority,omitempty"`
// 	SubjectUsers    []string          `json:"subject_users,omitempty"`
// 	SubjectGroups   []string          `json:"subject_groups,omitempty"`
// 	ResourceApps    []string          `json:"resource_apps,omitempty"`
// 	ResourcePaths   []string          `json:"resource_paths,omitempty"`
// 	ResourceMethods []string          `json:"resource_methods,omitempty"`
// 	Conditions      *PolicyConditions `json:"conditions,omitempty"`
// 	Enabled         *bool             `json:"enabled,omitempty"`
// }

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

// type MFACondition struct {
// 	Required bool   `json:"required"`
// 	MinLevel string `json:"min_level,omitempty"`
// }

// type DeviceCondition struct {
// 	Postures []string `json:"postures"`
// }

// type NetworkCondition struct {
// 	AllowedCountries []string `json:"allowed_countries,omitempty"`
// 	BlockedCountries []string `json:"blocked_countries,omitempty"`
// 	AllowedCIDRs     []string `json:"allowed_cidrs,omitempty"`
// 	BlockedCIDRs     []string `json:"blocked_cidrs,omitempty"`
// 	BlockTor         bool     `json:"block_tor,omitempty"`
// }

// type TimeCondition struct {
// 	ScheduleName string `json:"schedule_name"`
// }
