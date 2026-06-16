package org

import "time"

type CreateOrgRequest struct {
	Name string `json:"name" validate:"required"`
	Slug string `json:"slug" validate:"required,alphanum"`
}

type UpdateOrgRequest struct {
	Name string `json:"name" validate:"required"`
}

type UpdateOrgCustomDomainRequest struct {
	CustomDomain string `json:"custom_domain" validate:"required,url"`
}

type DomainVerifyRequest struct {
	DomainToken string `json:"domain_token" validate:"required"`
}

type OrgResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CustomDomain string    `json:"custom_domain,omitempty"`
	Slug      string    `json:"slug"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
