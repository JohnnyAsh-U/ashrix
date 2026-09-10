package gateway

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/database/store"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/events"
	pkica "github.com/JohnnyAsh-U/ashrix-api/internal/cp/pki_ca"

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
	repo       Repository
	eventRepo  events.Repository
	pkiRepo    pkica.Repository
	dispatcher *events.GatewayDispatcher
}

func NewService(repo Repository, pkiRepo pkica.Repository, eventRepo events.Repository, dispatcher *events.GatewayDispatcher) *Service {
	return &Service{repo: repo, pkiRepo: pkiRepo, eventRepo: eventRepo, dispatcher: dispatcher}
}

func buildGatewayInstruction(token string) GatewayInstruction {
	downloadCmd := "curl -L https://dl.ashrix.io/gateway/linux-amd64 -o ashrix-gateway && chmod +x ashrix-gateway"
	enrollCmd := fmt.Sprintf("./ashrix-gateway register \\\n  --token=%s", token)
	startCmd := "./ashrix-gateway start"

	return GatewayInstruction{
		DownloadCmd: downloadCmd,
		EnrollCmd:   enrollCmd,
		StartCmd:    startCmd,
	}
}

// mapToGatewayResponse converts a store.CreateGatewayRow to a GatewayResponse DTO.
func mapToGatewayResponse(g store.CreateGatewayRow, token string) GatewayResponse {
	resp := GatewayResponse{
		ID:             g.ID.String(),
		Name:           g.Name,
		OrgID:          g.OrgID.String(),
		Version:        g.Version.String,
		DeploymentType: g.DeploymentType,
		PublicURL:      g.PublicUrl,
		IPAddress:      g.IpAddress,
		Status:         g.Status,
		LogToCP:        g.LogToCp,
		Apps:           []GatewayAppSummary{},
		CreatedAt:      g.CreatedAt,
		Token:          token,
	}
	if token != "" {
		inst := buildGatewayInstruction(token)
		resp.DownloadCmd = inst.DownloadCmd
		resp.EnrollCmd = inst.EnrollCmd
		resp.StartCmd = inst.StartCmd
		resp.Instruction = &inst
	}
	if g.LastHeartbeat.Valid {
		resp.LastHeartBeat = &g.LastHeartbeat.Time
	}
	if g.EnrolledAt.Valid {
		resp.EnrolledAt = &g.EnrolledAt.Time
	}
	if g.RevokedAt.Valid {
		resp.RevokedAt = &g.RevokedAt.Time
	}
	return resp
}

// mapToGatewayResponse2 converts a store.Gateway to a GatewayResponse DTO.
func mapToGatewayResponse2(g store.Gateway, activeSessions int64, certificate store.ComponentCertificate, token string, currentPolicyVersion int64, apps []store.App) GatewayResponse {
	resp := GatewayResponse{
		ID:                    g.ID.String(),
		Name:                  g.Name,
		OrgID:                 g.OrgID.String(),
		Version:               g.Version.String,
		DeploymentType:        g.DeploymentType,
		PublicURL:             g.PublicUrl,
		IPAddress:             g.IpAddress,
		LogToCP:               g.LogToCp,
		Status:                g.Status,
		Uptime:                g.Uptime,
		NumberOfSessionActive: activeSessions,
		CurrentPolicyVersion:  currentPolicyVersion,
		CreatedAt:             g.CreatedAt,
		Token:                 token,
		CertificateIssuedAt:   &certificate.CreatedAt,
		CertificateExpiresAt:  &certificate.ExpiresAt,
	}
	if token != "" {
		inst := buildGatewayInstruction(token)
		resp.DownloadCmd = inst.DownloadCmd
		resp.EnrollCmd = inst.EnrollCmd
		resp.StartCmd = inst.StartCmd
		resp.Instruction = &inst
	}
	if g.LastHeartbeat.Valid {
		resp.LastHeartBeat = &g.LastHeartbeat.Time
	}
	if g.EnrolledAt.Valid {
		resp.EnrolledAt = &g.EnrolledAt.Time
	}
	if g.RevokedAt.Valid {
		resp.RevokedAt = &g.RevokedAt.Time
	}

	appSummaries := make([]GatewayAppSummary, 0, len(apps))
	for _, app := range apps {
		appSummaries = append(appSummaries, GatewayAppSummary{
			ID:           app.ID.String(),
			Name:         app.Name,
			Subdomain:    app.Subdomain,
			HealthStatus: app.HealthStatus,
		})
	}
	resp.Apps = appSummaries
	resp.NumberOfApps = len(appSummaries)

	return resp
}

// CreateGateway creates a new gateway.
func (s *Service) CreateGateway(ctx context.Context, name, IPAdress, PublicURL string, LogToCP bool) (GatewayResponse, *dto.AppError) {

	//Check if the Org is same as the Admin
	AdminOrgId := middleware.OrgIDFromCtx(ctx)
	AdminUUID, err := uuid.Parse(AdminOrgId)
	if err != nil {
		return GatewayResponse{}, dto.NewUnauthorizedError("OrgID Error")
	}

	//Generate a 6 chars token and hash
	token := utils.GenerateRandomString(6)

	params := store.CreateGatewayParams{
		OrgID:          AdminUUID,
		Name:           name,
		TokenHash:      utils.HashToken(token),
		DeploymentType: "hosted",
		PublicUrl:      PublicURL,
		IpAddress:      IPAdress,
		LogToCp:        LogToCP,
	}
	gateway, err := s.repo.CreateGateway(ctx, params)
	if err != nil {
		return GatewayResponse{}, dto.NewAppError(500, dto.CodeInternal, "Failed to create gateway", err.Error())
	}
	return mapToGatewayResponse(gateway, token), nil
}

// ListGatewaysByOrg lists all gateways for a given organization.
func (s *Service) ListGatewaysByOrg(ctx context.Context, orgID uuid.UUID) ([]GatewayResponse, *dto.AppError) {
	gateways, err := s.repo.ListGatewayByOrg(ctx, orgID)
	if err != nil {
		return nil, dto.NewAppError(500, dto.CodeInternal, "Failed to list gateways", err.Error())
	}

	latestPolicyVer, _ := s.repo.GetLatestPolicyVersion(ctx, orgID)

	var responses []GatewayResponse
	for _, g := range gateways {
		activeSessions, _ := s.repo.CountActiveUserSessionsByGateway(ctx, g.ID)
		apps, _ := s.repo.ListAppsByGateway(ctx, g.ID)
		cert, _ := s.pkiRepo.GetActiveComponentCert(ctx, store.GetActiveComponentCertParams{
			ComponentID:   pgtype.UUID{Bytes: g.ID, Valid: true},
			ComponentType: "gateway",
		})
		responses = append(responses, mapToGatewayResponse2(g, activeSessions, cert, "", latestPolicyVer, apps))
	}
	if responses == nil {
		responses = []GatewayResponse{}
	}
	return responses, nil
}

// GetGatewayByID returns a gateway owned by the authenticated organization.
func (s *Service) GetGatewayByID(ctx context.Context, id uuid.UUID) (GatewayResponse, *dto.AppError) {
	adminOrgID, err := uuid.Parse(middleware.OrgIDFromCtx(ctx))
	if err != nil {
		return GatewayResponse{}, dto.NewBadRequestError("Invalid Organization ID format")
	}

	gateway, err := s.repo.GetGatewayByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return GatewayResponse{}, dto.NewNotFoundError("Gateway Not Found")
		}
		return GatewayResponse{}, dto.NewAppError(500, dto.CodeInternal, "Failed to retrieve gateway", err.Error())
	}

	if gateway.OrgID != adminOrgID {
		return GatewayResponse{}, dto.NewUnauthorizedError("OrgID Error")
	}

	activeSessions, _ := s.repo.CountActiveUserSessionsByGateway(ctx, gateway.ID)
	apps, _ := s.repo.ListAppsByGateway(ctx, gateway.ID)
	certificate, _ := s.pkiRepo.GetActiveComponentCert(ctx, store.GetActiveComponentCertParams{
		ComponentID:   pgtype.UUID{Bytes: gateway.ID, Valid: true},
		ComponentType: "gateway",
	})
	policyVersion, _ := s.repo.GetLatestPolicyVersion(ctx, gateway.OrgID)

	return mapToGatewayResponse2(gateway, activeSessions, certificate, "", policyVersion, apps), nil
}

// ReEnrollGateway re-create a gateway.
func (s *Service) ReCreateGateway(ctx context.Context, id uuid.UUID, name, IPAdress, PublicURL string, LogtoCP bool) (GatewayResponse, *dto.AppError) {

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

	params := store.ReCreateGatewayParams{
		ID:        id,
		Name:      name, // Assuming name can be updated during re-enrollment
		TokenHash: utils.HashToken(token),
		PublicUrl: PublicURL,
		IpAddress: IPAdress,
		LogToCp:   LogtoCP,
	}

	gateway, err := s.repo.ReCreateGateway(ctx, params)

	if err != nil {
		return GatewayResponse{}, dto.NewAppError(500, dto.CodeInternal, "Failed to re-enroll gateway", err.Error())
	}

	_ = s.eventRepo.AckGatewayEvents(ctx, store.AckGatewayEventsParams{GatewayID: id})

	//Revoke any Existing cert and disconnect the gateway
	activeCert, err := s.pkiRepo.GetActiveComponentCert(ctx, store.GetActiveComponentCertParams{
		ComponentType: "gateway",
		ComponentID:   pgtype.UUID{Valid: true, Bytes: id},
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// No active cert to revoke, but we can return the gateway response as success
			return mapToGatewayResponse2(gateway, 0, store.ComponentCertificate{}, token, 0, nil), nil
		}
		return GatewayResponse{}, dto.NewAppError(500, dto.CodeInternal, "Failed to retrieve active component certificate", err.Error())
	}

	// Revoke component cert
	paramsRevokeCert := store.RevokeCompCertParams{
		ComponentID:   pgtype.UUID{Valid: true, Bytes: id},
		ComponentType: "gateway",
		RevokeReason:  pgtype.Text{String: "cessationOfOperation", Valid: true},
	}
	_, err = s.pkiRepo.RevokeComponentCert(ctx, paramsRevokeCert)
	if err != nil {
		return GatewayResponse{}, dto.NewAppError(500, dto.CodeInternal, "Failed to revoke component certificate", err.Error())
	}

	// Create CRL Entry
	_, err = s.pkiRepo.CreateCRLEntry(ctx, store.CreateCRLEntryParams{
		CertID:       activeCert.ID,
		SerialNumber: activeCert.SerialNumber,
		Reason:       "cessationOfOperation",
	})
	if err != nil {
		return GatewayResponse{}, dto.NewAppError(500, dto.CodeInternal, "Failed to create CRL entry", err.Error())
	}

	// Dispatch RevokeGatewayCertCmd to the gateway if connected

	payload := events.CommandJob{
		Type:      events.CmdRevokeGateway,
		GatewayID: id.String(),
	}

	_, err = s.eventRepo.CreateEvent(ctx, payload)

	if err != nil {
		return GatewayResponse{}, dto.NewAppError(500, dto.CodeInternal, err.Error(), nil)
	}

	//Get all the active CRLs
	adminOrgUUID, _ := uuid.Parse(AdminOrgId)
	activeCRLs, _ := s.pkiRepo.GetCRLEntryByOrg(ctx, adminOrgUUID)

	var revokedSerials []string
	for _, crl := range activeCRLs {
		revokedSerials = append(revokedSerials, crl.SerialNumber)
	}

	payloadRevokedSerials := events.CommandJob{
		Type:                 events.CmdCrlSync,
		GatewayID:            id.String(),
		RevokedSerialNumbers: revokedSerials,
	}

	_, err = s.eventRepo.CreateEvent(ctx, payloadRevokedSerials)

	if err != nil {
		return GatewayResponse{}, dto.NewAppError(500, dto.CodeInternal, err.Error(), nil)
	}

	s.dispatcher.Wakeup(id.String())

	return mapToGatewayResponse2(gateway, 0, store.ComponentCertificate{}, token, 0, nil), nil
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
	gatewayCRT, err := signer.IssueCert(certReq, 90*24*time.Hour, gateway.ID.String(), gateway.PublicUrl)
	if err != nil {
		return gen.GatewayEnrollResponse{}, dto.NewBadRequestError("Error in signing CSR: " + err.Error())
	}

	// Get active intermediate CA certificate from DB to retrieve its ID
	caCertRecord, err := s.pkiRepo.GetActiveCACert(ctx, store.GetActiveCACertParams{
		Name: "Ashrix Intermediate CA",
		Type: "intermediate",
	})
	if err != nil {
		return gen.GatewayEnrollResponse{}, dto.NewAppError(500, dto.CodeInternal, "Failed to retrieve active Intermediate CA", err.Error())
	}

	// Register component cert
	_, err = s.pkiRepo.CreateComponentCert(ctx, store.RegisterCompCertParams{
		OrgID:         pgtype.UUID{Valid: true, Bytes: gateway.OrgID},
		ComponentType: "gateway",
		ComponentID:   pgtype.UUID{Valid: true, Bytes: gateway.ID},
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

	//Get the CP Public key from the Cert in the db
	cPCert, err := s.pkiRepo.GetActiveComponentCertByType(ctx, "cp")
	if err != nil {
		return gen.GatewayEnrollResponse{}, dto.NewAppError(500, dto.CodeInternal, "Failed to retrieve active CP certificate", err.Error())
	}

	pubkey, err := pki_utils.ParsePublicKeyFromCert([]byte(cPCert.CertPem))
	if err != nil {
		return gen.GatewayEnrollResponse{}, dto.NewAppError(500, dto.CodeInternal, "Failed to retrieve active CP certificate", "invalid cert PEM: "+cPCert.CertPem)
	}

	return gen.GatewayEnrollResponse{
		GatewayId:   gateway.ID.String(),
		TenantId:    gateway.OrgID.String(),
		GatewayName: gateway.Name,
		GatewayUrl:  gateway.PublicUrl,
		CpPubKey:    string(pki_utils.MarshalPubKey(pubkey)),
		Certificate: string(pki_utils.MarshalCert(gatewayCRT)),
		TrustBundle: string(signer.TrustBundle()),
		LogToCp:     gateway.LogToCp,
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
	gateway, err := s.repo.GetActiveGatewayByID(ctx, gatewayID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return gen.GatewayRenewCertResponse{}, dto.NewNotFoundError("Gateway Not Found")
		}
		return gen.GatewayRenewCertResponse{}, dto.NewAppError(500, dto.CodeInternal, "Failed to retrieve gateway", err.Error())
	}

	// get active cert
	activeCert, err := s.pkiRepo.GetActiveComponentCert(ctx, store.GetActiveComponentCertParams{
		ComponentType: "gateway",
		ComponentID:   pgtype.UUID{Valid: true, Bytes: gatewayID},
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
	verified := pki_utils.VerifyPossessionProof(pubkey, []byte(csr), gatewayID.String(), timestamp, signature)
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
	caCertRecord, err := s.pkiRepo.GetActiveCACert(ctx, store.GetActiveCACertParams{
		Name: "Ashrix Intermediate CA",
		Type: "intermediate",
	})
	if err != nil {
		return gen.GatewayRenewCertResponse{}, dto.NewAppError(500, dto.CodeInternal, "Failed to retrieve active Intermediate CA", err.Error())
	}

	//Revoke the current cert of the gateway
	_, err = s.pkiRepo.RevokeComponentCert(ctx, store.RevokeCompCertParams{
		ComponentID:   pgtype.UUID{Valid: true, Bytes: gatewayID},
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
	gatewayCRT, err := signer.IssueCert(certReq, 90*24*time.Hour, gatewayID.String(), gateway.PublicUrl)
	if err != nil {
		return gen.GatewayRenewCertResponse{}, dto.NewBadRequestError("Error in signing CSR: " + err.Error())
	}

	//Register the cert
	_, err = s.pkiRepo.CreateComponentCert(ctx, store.RegisterCompCertParams{
		OrgID:         pgtype.UUID{Valid: true, Bytes: gateway.OrgID},
		ComponentID:   pgtype.UUID{Valid: true, Bytes: gatewayID},
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
		GatewayId:   gatewayID.String(),
		Certificate: string(pki_utils.MarshalCert(gatewayCRT)),
		TrustBundle: string(signer.TrustBundle()),
		LogToCp:     gateway.LogToCp,
		ExpiresAt:   timestamppb.New(gatewayCRT.NotAfter),
	}, nil

}


// RevokeGateway revokes an entire gateway.
func (s *Service) RevokeGateway(ctx context.Context, id uuid.UUID, revokeReason string) (GatewayResponse, *dto.AppError) {
	// Retrieve the admin's OrgID from context
	adminOrgIDStr := middleware.OrgIDFromCtx(ctx)
	adminOrgID, parseErr := uuid.Parse(adminOrgIDStr)
	if parseErr != nil {
		return GatewayResponse{}, dto.NewBadRequestError("Invalid Organization ID format")
	}

		// Validate the revocation reason against DB check constraints
	switch revokeReason {
	case "keyCompromise", "superseded", "cessationOfOperation", "affiliationChanged":
		// valid reason
	default:
		return GatewayResponse{}, dto.NewBadRequestError("Invalid revocation reason")
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

	// Revoke the active component certificate if it exists
	activeCert, err := s.pkiRepo.GetActiveComponentCert(ctx, store.GetActiveComponentCertParams{
		ComponentType: "gateway",
		ComponentID:   pgtype.UUID{Valid: true, Bytes: id},
	})
	if err == nil {
		// Cert found, revoke it
		_, err = s.pkiRepo.RevokeComponentCert(ctx, store.RevokeCompCertParams{
			ComponentID:   pgtype.UUID{Valid: true, Bytes: id},
			ComponentType: "gateway",
			RevokeReason:  pgtype.Text{String: "cessationOfOperation", Valid: true},
		})
		if err == nil {
			// Write to CRL entries
			_, _ = s.pkiRepo.CreateCRLEntry(ctx, store.CreateCRLEntryParams{
				CertID:       activeCert.ID,
				SerialNumber: activeCert.SerialNumber,
				Reason:       "cessationOfOperation",
			})
		}
	}

	// Mark the gateway as revoked (offline) in the database
	params := store.RevokeGatewayParams{
		ID:    id,
		OrgID: adminOrgID,
	}
	_, err = s.repo.RevokeGateway(ctx, params)
	if err != nil {
		return GatewayResponse{}, dto.NewAppError(500, dto.CodeInternal, "Failed to revoke gateway record", err.Error())
	}


	payload := events.CommandJob{
		Type:      events.CmdRevokeGateway,
		GatewayID: id.String(),
	}

	_, err = s.eventRepo.CreateEvent(ctx, payload)

	if err != nil {
		return GatewayResponse{}, dto.NewAppError(500, dto.CodeInternal, err.Error(), nil)
	}

	//Get all the active CRLs
	activeCRLs, _ := s.pkiRepo.GetCRLEntryByOrg(ctx, adminOrgID)

	var revokedSerials []string
	for _, crl := range activeCRLs {
		revokedSerials = append(revokedSerials, crl.SerialNumber)
	}

	payloadCrlSync := events.CommandJob{
		Type:                 events.CmdCrlSync,
		GatewayID:            id.String(),
		RevokedSerialNumbers: revokedSerials,
	}

	_, err = s.eventRepo.CreateEvent(ctx, payloadCrlSync)

	if err != nil {
		return GatewayResponse{}, dto.NewAppError(500, dto.CodeInternal, err.Error(), nil)
	}

	s.dispatcher.Wakeup(id.String())

	// Fetch updated gateway status
	updatedGateway, err := s.repo.GetGatewayByID(ctx, id)
	if err != nil {
		return GatewayResponse{}, dto.NewAppError(500, dto.CodeInternal, "Failed to retrieve gateway", err.Error())
	}

	return mapToGatewayResponse2(updatedGateway, 0, store.ComponentCertificate{}, "", 0, nil), nil
}

// Send Rotate Gateway Cert Cmd
func (s *Service) RotateGatewayCert(ctx context.Context, id uuid.UUID) (GatewayResponse, *dto.AppError) {
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

	payload := events.CommandJob{
		Type:      events.CmdRotateGatewayCert,
		GatewayID: id.String(),
	}

	_, err = s.eventRepo.CreateEvent(ctx, payload)

	if err != nil {
		return GatewayResponse{}, dto.NewAppError(500, dto.CodeInternal, err.Error(), nil)
	}

	s.dispatcher.Wakeup(id.String())

	// Fetch updated gateway
	updatedGateway, err := s.repo.GetGatewayByID(ctx, id)
	if err != nil {
		return GatewayResponse{}, dto.NewAppError(500, dto.CodeInternal, "Failed to retrieve gateway", err.Error())
	}

	return mapToGatewayResponse2(updatedGateway, 0, store.ComponentCertificate{}, "", 0, nil), nil
}

// DrainGateway sets status to 'draining' and pushes DrainGatewayCmd.
func (s *Service) DrainGateway(ctx context.Context, id uuid.UUID) (GatewayResponse, *dto.AppError) {
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

	// Update status to 'draining'
	_, err = s.repo.UpdateGatewayStatus(ctx, store.UpdateGatewayStatusParams{
		ID:     id,
		Status: "draining",
	})
	if err != nil {
		return GatewayResponse{}, dto.NewAppError(500, dto.CodeInternal, "Failed to update gateway status to draining", err.Error())
	}

	// Push DrainGatewayCmd if connected
	payload := events.CommandJob{
		Type:      events.CmdDrainGateway,
		GatewayID: id.String(),
	}

	_, err = s.eventRepo.CreateEvent(ctx, payload)

	if err != nil {
		return GatewayResponse{}, dto.NewAppError(500, dto.CodeInternal, err.Error(), nil)
	}

	s.dispatcher.Wakeup(id.String())

	// Fetch updated gateway
	updatedGateway, err := s.repo.GetGatewayByID(ctx, id)
	if err != nil {
		return GatewayResponse{}, dto.NewAppError(500, dto.CodeInternal, "Failed to retrieve gateway", err.Error())
	}

	return mapToGatewayResponse2(updatedGateway, 0, store.ComponentCertificate{}, "", 0, nil), nil
}


func CheckAndUpdateGatewayStatus (ctx context.Context, gatewayRepo Repository, log *slog.Logger){
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	count, err := gatewayRepo.MarkOfflineStaleGateways(ctx, "2")

	if err != nil {
		log.Error("[HealthCheck] failed %v", slog.Any("err",err))
		return
	}
	if count > 0 {
		log.Info("[HealthCheck] gateway marked offline", slog.Any("count", count))
	}
}