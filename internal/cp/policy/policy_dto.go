package policy

import (
	"time"

	"github.com/google/uuid"
)

type Effect string

const (
	EffectAllow Effect = "ALLOW"
	EffectDeny  Effect = "DENY"
)

type Subject struct {
	Type  string `json:"type" example:"group" enums:"group,user"`  // "group" or "user"
	Value string `json:"value" example:"engineering"` // "engineering", "u-123"
}

type Resource struct {
	Type  string `json:"type" example:"app" enums:"app,path,method"`
	Value string `json:"value" example:"jenkins-prod"`
}

type Conditions struct {
	MFA     *MFACondition     `json:"mfa,omitempty"`
	Device  *DeviceCondition  `json:"device,omitempty"`
	Network *NetworkCondition `json:"network,omitempty"`
	Time    *TimeCondition    `json:"time,omitempty"`
}

type MFACondition struct {
	Required bool   `json:"required" example:"true"`
	MinLevel string `json:"min_level,omitempty" example:"totp"`
}

type DeviceCondition struct {
	Postures []string `json:"postures" example:"[\"compliant\"]"`
}

type NetworkCondition struct {
	AllowedCountries []string `json:"allowed_countries,omitempty" example:"[\"CI\",\"GH\"]"`
	BlockedCountries []string `json:"blocked_countries,omitempty"`
	AllowedCIDRs     []string `json:"allowed_cidrs,omitempty" example:"[\"102.68.0.0/16\"]"`
	BlockedCIDRs     []string `json:"blocked_cidrs,omitempty"`
	BlockTor         bool     `json:"block_tor,omitempty" example:"true"`
}

type TimeCondition struct {
	ScheduleName string `json:"schedule_name,omitempty" example:"business-hours"`
}


type CreatePolicyRequest struct {
	Name        string        `json:"name" binding:"required,min=1,max=255"`
	Description string        `json:"description"`
	Effect      Effect        `json:"effect" binding:"required,oneof=ALLOW DENY"`
	Priority    int32         `json:"priority" binding:"min=0,max=100"`
	Subjects    []Subject  `json:"subjects"`
	Resources   []Resource `json:"resources"`
	Conditions  Conditions `json:"conditions"`
}


type UpdatePolicyRequest struct {
	Name        *string       `json:"name,omitempty" binding:"omitempty,min=1,max=255"`
	Description *string       `json:"description,omitempty"`
	Effect      *Effect       `json:"effect,omitempty" binding:"omitempty,oneof=ALLOW DENY"`
	Priority    *int32        `json:"priority,omitempty" binding:"omitempty,min=0,max=100"`
	Enabled     *bool         `json:"enabled,omitempty"`
	Subjects    []Subject  `json:"subjects,omitempty"`
	Resources   []Resource `json:"resources,omitempty"`
	Conditions  *Conditions `json:"conditions,omitempty"`
}



type Policy struct {
	ID          uuid.UUID     `json:"id"`
	OrgID       uuid.UUID     `json:"org_id"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Effect      Effect        `json:"effect"`
	Priority    int32         `json:"priority"`
	Enabled     bool          `json:"enabled"`
	Version     int64         `json:"version"`
	Sequence    int64         `json:"sequence"`
	CreatedBy   uuid.UUID     `json:"created_by"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
	Subjects    []Subject  `json:"subjects"`
	Resources   []Resource `json:"resources"`
	Conditions  Conditions `json:"conditions"`
}

type PolicyConditions struct {
	MFA     *MFACondition     `json:"mfa,omitempty"`
	Device  *DeviceCondition  `json:"device,omitempty"`
	Network *NetworkCondition `json:"network,omitempty"`
	Time    *TimeCondition    `json:"time,omitempty"`
}


type ListPoliciesQuery struct {
	Page   int    `form:"page" binding:"min=1" example:"1"`
	Limit  int    `form:"limit" binding:"min=1,max=100" example:"20"`
	SortBy string `form:"sort_by" example:"priority"`
	Order  string `form:"order" binding:"omitempty,oneof=asc desc" example:"desc"`
}

type ListPoliciesResponse struct {
	Items []Policy `json:"items"`
	Total int64    `json:"total"`
	Page  int        `json:"page"`
	Limit int        `json:"limit"`
}

