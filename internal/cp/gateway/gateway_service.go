package gateway

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/database/store"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/dto"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/middleware"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/pki"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/utils"
	pki_utils "github.com/JohnnyAsh-U/ashrix-api/pkg/pki"
	gen "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"google.golang.org/protobuf/types/known/timestamppb"
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
func (s *Service) CreateGateway(ctx context.Context, orgID uuid.UUID, name, IPAdress,PublicURL string) (GatewayResponse, *dto.AppError) {

	//Check if the Org is same as the Admin
	AdminOrgId := middleware.OrgIDFromCtx(ctx)
	if AdminOrgId != orgID.String() {
		return GatewayResponse{}, dto.NewUnauthorizedError("OrgID Error")
	}

	//Generate a 6 chars token and hash
	token := utils.GenerateRandomString(6)
	fmt.Println(token)

	params := store.CreateGatewayParams{
		OrgID:     orgID,
		Name:      name,
		TokenHash: utils.HashToken(token),
		Type: "ashrix_hosted",
		PublicUrl: PublicURL,
		IpAddress: IPAdress,
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
func (s *Service) ReCreateGateway(ctx context.Context, id uuid.UUID, name, IPAdress,PublicURL string) (GatewayResponse, *dto.AppError) {

	gatewayRes, err := s.repo.GetGatewayByID(ctx, id)

	if err != nil {
		return GatewayResponse{}, dto.NewNotFoundError("Not Found")
	}

	//Check if the Org is same as the Admin
	AdminOrgId := middleware.OrgIDFromCtx(ctx)
	if AdminOrgId != gatewayRes.OrgID.String() {
		return GatewayResponse{}, dto.NewUnauthorizedError("OrgID Error")
	}

	//Generate a 6 chars token and hash
	token := utils.GenerateRandomString(6)
	fmt.Println(token)

	params := store.ReCreateGatewayParams{
		ID:        id,
		Name:      name, // Assuming name can be updated during re-enrollment
		TokenHash: utils.HashToken(token),
		PublicUrl: PublicURL,
		IpAddress: PublicURL,
	}
	gateway, err := s.repo.ReCreateGateway(ctx, params)
	if err != nil {
		return GatewayResponse{}, dto.NewAppError(500, dto.CodeInternal, "Failed to re-enroll gateway", err.Error())
	}
	return mapToGatewayResponse(gateway), nil
}

// EnrollGateway enrolls a gateway using a token hash and CSR.
func (s *Service) EnrollGateway(ctx context.Context, token, csr string, signer pki.CASigner) (gen.GatewayEnrollResponse, *dto.AppError) {
	// Get the Gateway by using the token
	gateway, err := s.repo.GetGatewayByTokenHash(ctx, utils.HashToken(token))
	if err != nil {
		return gen.GatewayEnrollResponse{}, dto.NewNotFoundError("Gateway Not Found")
	}

	// Parse CSR
	certReq, err := pki_utils.ParseCSR([]byte(csr))
	if err != nil {
		return gen.GatewayEnrollResponse{}, dto.NewBadRequestError("Not Valid CSR: " + err.Error())
	}


	//Validate the CSR fields
	if len(certReq.Subject.OrganizationalUnit) == 0 || certReq.Subject.OrganizationalUnit[0] != "Gateway" {
		return gen.GatewayEnrollResponse{}, dto.NewUnauthorizedError("Not a gateway")
	}

	// Sign the cert
	gatewayCRT, err := signer.IssueCert(certReq, 90*24*time.Hour, gateway.ID.String())
	if err != nil {
		return gen.GatewayEnrollResponse{}, dto.NewBadRequestError("Error in signing CSR: " + err.Error())
	}

	// Get active intermediate CA certificate from DB to retrieve its ID
	caCertRecord, err := s.repo.GetActiveCACert(ctx, store.GetActiveCACertParams{
		Name: "Ashrix Intermediate CA",
		Type: "intermediate",
	})
	if err != nil {
		return gen.GatewayEnrollResponse{}, dto.NewAppError(500, dto.CodeInternal, "Failed to retrieve active Intermediate CA", err.Error())
	}

	// Register component cert
	_, err = s.repo.CreateGatewayCert(ctx, store.RegisterCompCertParams{
		OrgID:         gateway.OrgID,
		ComponentType: "gateway",
		ComponentID:   gateway.ID,
		CaID:          caCertRecord.ID,
		CertPem:       string(pki_utils.MarshalCert(gatewayCRT)),
		SerialNumber:  gatewayCRT.SerialNumber.String(),
		Subject:       gatewayCRT.Subject.CommonName,
		San:           gatewayCRT.DNSNames,
		IssuedAt:      gatewayCRT.NotBefore,
		ExpiresAt:     gatewayCRT.NotAfter,
		RotationOf:    pgtype.UUID{Valid: false},
	})
	if err != nil {
		return gen.GatewayEnrollResponse{}, dto.NewAppError(500, dto.CodeInternal, "Failed to register gateway certificate", err.Error())
	}

	// Update gateway status to healthy/enrolled
	_, err = s.repo.EnrollGatewayUsingTokenHash(ctx, utils.HashToken(token))
	if err != nil {
		return gen.GatewayEnrollResponse{}, dto.NewAppError(500, dto.CodeInternal, "Failed to enroll gateway", err.Error())
	}

	return gen.GatewayEnrollResponse{
		GatewayId: gateway.ID.String(),
		Certificate: string(pki_utils.MarshalCert(gatewayCRT)),
		TrustBundle: string(signer.TrustBundle()),
		ExpiresAt:   timestamppb.New(gatewayCRT.NotAfter),
	}, nil
}

func (s *Service) RenewGatewayCert(ctx context.Context, gatewayID uuid.UUID, signature string, csr string, timestamp int64, signer pki.CASigner) (gen.GatewayRenewCertResponse, *dto.AppError) {
	//Check timestamp is within 60 secsy
	timeNow := time.Now().Unix()
	if timeNow-timestamp > 60 {
		return gen.GatewayRenewCertResponse{}, dto.NewUnauthorizedError("Timestamp Expired")
	}
	
	// get gateway
	gateway, err := s.repo.GetGatewayByID(ctx, gatewayID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return gen.GatewayRenewCertResponse{}, dto.NewNotFoundError("Gateway Not Found")
		}
		return gen.GatewayRenewCertResponse{}, dto.NewAppError(500, dto.CodeInternal, "Failed to retrieve gateway", err.Error())
	}

	// get active cert
	activeCert, err := s.repo.GetActiveComponentCert(ctx, store.GetActiveComponentCertParams{
		ComponentType: "gateway",
		ComponentID:   gatewayID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return gen.GatewayRenewCertResponse{}, dto.NewNotFoundError("Active Component Certificate Not Found")
		}
		return gen.GatewayRenewCertResponse{}, dto.NewAppError(500, dto.CodeInternal, "Failed to retrieve active component certificate", err.Error())
	}

	certPem := []byte(activeCert.CertPem)
	pubkey, err := pki_utils.ParsePublicKeyFromCert(certPem)
	if err != nil {
		return gen.GatewayRenewCertResponse{}, dto.NewAppError(500, dto.CodeInternal, "Failed to parse active component certificate", err.Error())
	}

	// verify signature
	verified := pki_utils.VerifyPossessionProof	(pubkey, []byte(csr), gatewayID.String(), timestamp, signature)
	if !verified {
		return gen.GatewayRenewCertResponse{}, dto.NewUnauthorizedError("Signature verification failed")
	}

		//Check timestamp is within 60 secsy
	timeNow = time.Now().Unix()
	if timeNow-timestamp > 60 {
		return gen.GatewayRenewCertResponse{}, dto.NewUnauthorizedError("Timestamp Expired")
	}


	// Issue new cert
	// get active intermediate CA
	caCertRecord, err := s.repo.GetActiveCACert(ctx, store.GetActiveCACertParams{
		Name: "Ashrix Intermediate CA",
		Type: "intermediate",
	})
	if err != nil {
		return gen.GatewayRenewCertResponse{}, dto.NewAppError(500, dto.CodeInternal, "Failed to retrieve active Intermediate CA", err.Error())
	}

	//Revoke the current cert of the gateway
	_, err = s.repo.RevokeGatewayCert(ctx, store.RevokeCompCertParams{
		ComponentID:   gatewayID,
		ComponentType: "gateway",
		RevokeReason:  pgtype.Text{String: "superseded", Valid: true},
	})

	if err != nil {
		return gen.GatewayRenewCertResponse{}, dto.NewAppError(500, dto.CodeInternal, "Failed to revoke gateway certificate", err.Error())
	}

	//Parse CSR
	certReq, err := pki_utils.ParseCSR([]byte(csr))
	if err != nil {
		return gen.GatewayRenewCertResponse{}, dto.NewBadRequestError("Not Valid CSR: " + err.Error())
	}

	//Validate the csr fields
	if len(certReq.Subject.OrganizationalUnit) == 0 || certReq.Subject.OrganizationalUnit[0] != "Gateway" {
		return gen.GatewayRenewCertResponse{}, dto.NewUnauthorizedError("Not a gateway")
	}


	// Issue new cert
	gatewayCRT, err := signer.IssueCert(certReq, 90*24*time.Hour, gatewayID.String())
	if err != nil {
		return gen.GatewayRenewCertResponse{}, dto.NewBadRequestError("Error in signing CSR: " + err.Error())
	}

	//Register the cert
	_, err = s.repo.CreateGatewayCert(ctx, store.RegisterCompCertParams{
		OrgID:         gateway.OrgID,
		ComponentID:   gatewayID,
		ComponentType: "gateway",
		CaID:          caCertRecord.ID,
		CertPem:       string(pki_utils.MarshalCert(gatewayCRT)),
		SerialNumber:  gatewayCRT.SerialNumber.String(),
		Subject:       gatewayCRT.Subject.CommonName,
		San:           gatewayCRT.DNSNames,
		IssuedAt:      gatewayCRT.NotBefore,
		ExpiresAt:     gatewayCRT.NotAfter,
		RotationOf:    pgtype.UUID{Valid: false},
	})
	if err != nil {
		return gen.GatewayRenewCertResponse{}, dto.NewAppError(500, dto.CodeInternal, "Failed to register gateway certificate", err.Error())
	}

	return gen.GatewayRenewCertResponse{
		Certificate: string(pki_utils.MarshalCert(gatewayCRT)),
		TrustBundle: string(signer.TrustBundle()),
		ExpiresAt:   timestamppb.New(gatewayCRT.NotAfter),
	}, nil

}

// RevokeGatewayCert revokes a gateway certificate.
// It assumes componentID is the gateway's ID.
func (s *Service) RevokeGatewayCert(ctx context.Context, gatewayID uuid.UUID, componentType, revokeReason string) (GatewayResponse, *dto.AppError) {
	// Retrieve the admin's OrgID from context
	adminOrgIDStr := middleware.OrgIDFromCtx(ctx)

	// Validate the revocation reason against DB check constraints
	switch revokeReason {
	case "keyCompromise", "superseded", "cessationOfOperation", "affiliationChanged":
		// valid reason
	default:
		return GatewayResponse{}, dto.NewBadRequestError("Invalid revocation reason")
	}

	// Fetch the gateway first to verify it exists
	gateway, err := s.repo.GetGatewayByID(ctx, gatewayID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return GatewayResponse{}, dto.NewNotFoundError("Gateway Not Found")
		}
		return GatewayResponse{}, dto.NewAppError(500, dto.CodeInternal, "Failed to retrieve gateway", err.Error())
	}

	// Verify that the gateway belongs to the admin's organization
	if adminOrgIDStr != gateway.OrgID.String() {
		return GatewayResponse{}, dto.NewUnauthorizedError("OrgID Error")
	}

	// Fetch active component certificate
	activeCert, err := s.repo.GetActiveComponentCert(ctx, store.GetActiveComponentCertParams{
		ComponentType: componentType,
		ComponentID:   gatewayID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// No active cert to revoke, but we can return the gateway response as success
			return mapToGatewayResponse(gateway), nil
		}
		return GatewayResponse{}, dto.NewAppError(500, dto.CodeInternal, "Failed to retrieve active component certificate", err.Error())
	}

	// Revoke component cert
	params := store.RevokeCompCertParams{
		ComponentID:   gatewayID,
		ComponentType: componentType,
		RevokeReason:  pgtype.Text{String: revokeReason, Valid: true},
	}
	_, err = s.repo.RevokeGatewayCert(ctx, params)
	if err != nil {
		return GatewayResponse{}, dto.NewAppError(500, dto.CodeInternal, "Failed to revoke component certificate", err.Error())
	}

	// Create CRL Entry
	_, err = s.repo.CreateCRLEntry(ctx, store.CreateCRLEntryParams{
		CertID:       activeCert.ID,
		SerialNumber: activeCert.SerialNumber,
		Reason:       revokeReason,
	})
	if err != nil {
		return GatewayResponse{}, dto.NewAppError(500, dto.CodeInternal, "Failed to create CRL entry", err.Error())
	}

	// Create Revocation record
	_, err = s.repo.CreateRevocation(ctx, store.CreateRevocationParams{
		OrgID:     gateway.OrgID,
		Type:      componentType,
		TargetID:  gatewayID.String(),
		Reason:    pgtype.Text{String: revokeReason, Valid: true},
		ExpiresAt: time.Now().Add(365 * 24 * time.Hour), // 1 year expiry
	})
	if err != nil {
		return GatewayResponse{}, dto.NewAppError(500, dto.CodeInternal, "Failed to write revocation event", err.Error())
	}

	// Fetch updated gateway status
	updatedGateway, err := s.repo.GetGatewayByID(ctx, gatewayID)
	if err != nil {
		return GatewayResponse{}, dto.NewAppError(500, dto.CodeInternal, "Failed to retrieve gateway", err.Error())
	}

	return mapToGatewayResponse(updatedGateway), nil
}

// RevokeGateway revokes an entire gateway.
func (s *Service) RevokeGateway(ctx context.Context, id uuid.UUID) (GatewayResponse, *dto.AppError) {
	// Retrieve the admin's OrgID from context
	adminOrgIDStr := middleware.OrgIDFromCtx(ctx)
	adminOrgID, parseErr := uuid.Parse(adminOrgIDStr)
	if parseErr != nil {
		return GatewayResponse{}, dto.NewBadRequestError("Invalid Organization ID format")
	}

	// Fetch the gateway to verify it exists
	gateway, err := s.repo.GetGatewayByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return GatewayResponse{}, dto.NewNotFoundError("Gateway Not Found")
		}
		return GatewayResponse{}, dto.NewAppError(500, dto.CodeInternal, "Failed to retrieve gateway", err.Error())
	}

	// Verify that the gateway belongs to the admin's organization
	if adminOrgID != gateway.OrgID {
		return GatewayResponse{}, dto.NewUnauthorizedError("OrgID Error")
	}

	// Mark the gateway as revoked (offline) in the database
	params := store.RevokeGatewayParams{
		ID:    id,
		OrgID: adminOrgID,
	}
	revokedGateway, err := s.repo.RevokeGateway(ctx, params)
	if err != nil {
		return GatewayResponse{}, dto.NewAppError(500, dto.CodeInternal, "Failed to revoke gateway record", err.Error())
	}

	// Revoke the active component certificate if it exists
	activeCert, err := s.repo.GetActiveComponentCert(ctx, store.GetActiveComponentCertParams{
		ComponentType: "gateway",
		ComponentID:   id,
	})
	if err == nil {
		// Cert found, revoke it
		_, err = s.repo.RevokeGatewayCert(ctx, store.RevokeCompCertParams{
			ComponentID:   id,
			ComponentType: "gateway",
			RevokeReason:  pgtype.Text{String: "cessationOfOperation", Valid: true},
		})
		if err == nil {
			// Write to CRL entries
			_, _ = s.repo.CreateCRLEntry(ctx, store.CreateCRLEntryParams{
				CertID:       activeCert.ID,
				SerialNumber: activeCert.SerialNumber,
				Reason:       "cessationOfOperation",
			})
		}
	}

	// Write revocation event
	_, err = s.repo.CreateRevocation(ctx, store.CreateRevocationParams{
		OrgID:     gateway.OrgID,
		Type:      "gateway",
		TargetID:  id.String(),
		Reason:    pgtype.Text{String: "cessationOfOperation", Valid: true},
		ExpiresAt: time.Now().Add(365 * 24 * time.Hour), // 1 year expiry
	})
	if err != nil {
		return GatewayResponse{}, dto.NewAppError(500, dto.CodeInternal, "Failed to log gateway revocation event", err.Error())
	}

	return mapToGatewayResponse(revokedGateway), nil
}


