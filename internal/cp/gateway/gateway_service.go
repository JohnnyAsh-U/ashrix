package gateway

import (
	"context"
	"encoding/pem"
	"fmt"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/database/store"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/dto"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/middleware"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/pki"
	pki_utils "github.com/JohnnyAsh-U/ashrix-api/pkg/pki"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/utils"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// mapToGatewayResponse converts a store.Gateway to a GatewayResponse DTO.
func mapToGatewayResponse(g store.Gateway) GatewayResponse {
	resp := GatewayResponse{
		ID:        g.ID.String(),
		Name:      g.Name,
		OrgID:     g.OrgID.String(),
		Version:   g.Version.String,
		Status:    g.Status,
		CreatedAt: g.CreatedAt,
	}
	if g.LastHeartbeat.Valid {
		resp.LastHeartBeat = g.LastHeartbeat.Time
	}
	if g.EnrolledAt.Valid {
		resp.EnrolledAt = g.EnrolledAt.Time
	}
	if g.RevokedAt.Valid {
		resp.RevokedAt = g.RevokedAt.Time
	}
	return resp
}

// CreateGateway creates a new gateway.
func (s *Service) CreateGateway(ctx context.Context, orgID uuid.UUID, name string) (GatewayResponse, *dto.AppError) {

	//Check if the Org is same as the Admin
	AdminOrgId :=  middleware.OrgIDFromCtx(ctx)
	if AdminOrgId != orgID.String(){
		return GatewayResponse{}, dto.NewUnauthorizedError("OrgID Error")
	}

	//Generate a 6 chars token and hash
	token := utils.GenerateRandomString(6)
	fmt.Println(token)

	params := store.CreateGatewayParams{
		OrgID: orgID,
		Name:  name,
		TokenHash: utils.HashToken(token),
	}
	gateway, err := s.repo.CreateGateway(ctx, params)
	if err != nil {
		return GatewayResponse{}, dto.NewAppError(500, dto.CodeInternal, "Failed to create gateway", err.Error())
	}
	return mapToGatewayResponse(gateway), nil
}


// ListGatewaysByOrg lists all gateways for a given organization.
func (s *Service) ListGatewaysByOrg(ctx context.Context, orgID uuid.UUID) ([]GatewayResponse, *dto.AppError) {
	gateways, err := s.repo.ListGatewayByOrg(ctx, orgID)
	if err != nil {
		return nil, dto.NewAppError(500, dto.CodeInternal, "Failed to list gateways", err.Error())
	}

	var responses []GatewayResponse
	for _, g := range gateways {
		responses = append(responses, mapToGatewayResponse(g))
	}
	return responses, nil
}


// ReEnrollGateway re-create a gateway.
func (s *Service) ReCreateGateway(ctx context.Context, id uuid.UUID, name string) (GatewayResponse, *dto.AppError) {

	gatewayRes, err := s.repo.GetGatewayByID(ctx,id)

	if err != nil {
		return GatewayResponse{}, dto.NewNotFoundError("Not Found")
	}

	//Check if the Org is same as the Admin
	AdminOrgId :=  middleware.OrgIDFromCtx(ctx)
	if AdminOrgId != gatewayRes.OrgID.String(){
		return GatewayResponse{}, dto.NewUnauthorizedError("OrgID Error")
	}

	//Generate a 6 chars token and hash
	token := utils.GenerateRandomString(6)
	fmt.Println(token)

	params := store.ReCreateGatewayParams{
		ID:   id,
		Name: name, // Assuming name can be updated during re-enrollment
		TokenHash: utils.HashToken(token),
	}
	gateway, err := s.repo.ReCreateGateway(ctx, params)
	if err != nil {
		return GatewayResponse{}, dto.NewAppError(500, dto.CodeInternal, "Failed to re-enroll gateway", err.Error())
	}
	return mapToGatewayResponse(gateway), nil
}



// EnrollGateway enrolls a gateway using a token hash and CSR.
func (s *Service) EnrollGateway(ctx context.Context, token, csr string, signer pki.CASigner) (EnrollResponse, *dto.AppError) {
	// In a real scenario, this would involve a Certificate Authority (CA) client
	// to sign the CSR and issue a certificate. For this implementation,
	// we'll simulate the certificate generation and return dummy data.
	// Get the Gatewayby using the token
	gateway, err := s.repo.GetGatewayByTokenHash(ctx, utils.HashToken(token))
	if err != nil {
		// TODO: Handle specific errors like token not found, already enrolled, etc.
		return EnrollResponse{}, dto.NewNotFoundError("Gateway Not Found")
	}


	//Sign the cert and register the comp cert
	block, _ := pem.Decode([]byte(csr))
	if block == nil {
		return EnrollResponse{}, dto.NewBadRequestError("Not Valid CSR")
	}

	if block.Type != "CERTIFICATE REQUEST" {
		return EnrollResponse{}, dto.NewBadRequestError("Not Valid CSR")
	}

	// gatewayCRT, err := signer.IssueCert(block.Bytes, 90 * 24 * time.Hour)

	if err != nil {
		return EnrollResponse{}, dto.NewBadRequestError("Error in signing CSR")
	}

	// gatewayCert, err := s.repo.CreateGatewayCert(ctx, store.RegisterCompCertParams{
	// 	OrgID: gateway.OrgID,
	// 	ComponentType: "gateway",
	// 	// CaID: signer,
	// 	CertPem: string(pki_utils.MarshalCert(gatewayCRT)),
	// 	SerialNumber: gatewayCRT.SerialNumber.String(),
	// 	Subject: gatewayCRT.Subject.CommonName,

	// })

	// })

	// gateway, err := s.repo.EnrollGatewayUsingTokenHash(ctx, token)



	if err != nil {
		// TODO: Handle specific errors like token not found, already enrolled, etc.
		return EnrollResponse{}, dto.NewAppError(500, dto.CodeInternal, "Failed to enroll gateway", err.Error())
	}

	// Placeholder for certificate generation/signing
	dummyCert := "---BEGIN CERTIFICATE---\n...dummy gateway cert...\n---END CERTIFICATE---"
	dummyCACert := "---BEGIN CERTIFICATE---\n...dummy CA cert...\n---END CERTIFICATE---"
	expiresAt := time.Now().Add(365 * 24 * time.Hour) // Example: 1 year expiry

	// _ = gateway // Use gateway to avoid unused variable error, though its fields aren't used for the EnrollResponse return.

	return EnrollResponse{
		Certificate: dummyCert,
		CACert:      dummyCACert,
		ExpiresAt:   expiresAt,
	}, nil
}


// RevokeGatewayCert revokes a gateway certificate.
// It assumes componentID is the gateway's ID.
func (s *Service) RevokeGatewayCert(ctx context.Context, gatewayID uuid.UUID, componentType, revokeReason string) (GatewayResponse, *dto.AppError) {
	params := store.RevokeCompCertParams{
		ComponentID:   gatewayID, // Assuming ComponentID is the Gateway ID
		ComponentType: componentType,
		RevokeReason:  pgtype.Text{String: revokeReason, Valid: true},
	}
	compCert, err := s.repo.RevokeGatewayCert(ctx, params)
	if err != nil {
		return GatewayResponse{}, dto.NewAppError(500, dto.CodeInternal, "Failed to revoke gateway certificate", err.Error())
	}

	// After revoking the certificate, fetch the updated gateway status.
	gateway, err := s.repo.GetGatewayByID(ctx, compCert.ComponentID) // Assuming compCert.ComponentID is the gateway ID
	if err != nil {
		// This case might indicate an inconsistency or a different expected flow.
		return GatewayResponse{}, dto.NewAppError(500, dto.CodeInternal, "Failed to retrieve gateway after certificate revocation", err.Error())
	}

	return mapToGatewayResponse(gateway), nil
}

// RevokeGateway revokes an entire gateway.
func (s *Service) RevokeGateway(ctx context.Context, id uuid.UUID) (GatewayResponse, *dto.AppError) {
	params := store.RevokeGatewayParams{
		ID:           id,
	}
	gateway, err := s.repo.RevokeGateway(ctx, params)
	if err != nil {
		return GatewayResponse{}, dto.NewAppError(500, dto.CodeInternal, "Failed to revoke gateway", err.Error())
	}
	return mapToGatewayResponse(gateway), nil
}


