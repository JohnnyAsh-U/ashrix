package connector

import (
	"context"
	"crypto/ecdsa"
	"crypto/x509"
	"database/sql"
	"encoding/pem"
	"errors"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/database/store"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/events"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/gateway"
	pkica "github.com/JohnnyAsh-U/ashrix-api/internal/cp/pki_ca"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/dto"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/middleware"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/pki"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/utils"
	pki_utils "github.com/JohnnyAsh-U/ashrix-api/pkg/pki"
	gen "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	proto "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Service struct {
	repo        Repository
	pkiRepo     pkica.Repository
	eventRepo   events.Repository
	gatewayRepo gateway.Repository
	dispatcher  *events.GatewayDispatcher
}

func NewService(repo Repository, pkiRepo pkica.Repository, eventRepo events.Repository, gatewayRepo gateway.Repository, dispatcher *events.GatewayDispatcher) *Service {
	return &Service{repo: repo, pkiRepo: pkiRepo, gatewayRepo: gatewayRepo, eventRepo: eventRepo, dispatcher: dispatcher}
}

// mapToConnectorResponse converts a store.Connector to a ConnectorResponse DTO.
func mapToConnectorResponse(g store.Connector, token string) ConnectorResponse {
	resp := ConnectorResponse{
		ID:           g.ID.String(),
		Name:         g.Name,
		OrgID:        g.OrgID.String(),
		GatewayID:    g.GatewayID.String(),
		ActiveStream: g.ActiveStreams,
		OpenSock:     g.OpenSock,
		Status:       g.Status,
		Apps:         []ConnectorAppSummaryResponse{},
		Token:        token,
		CreatedAt:    g.CreatedAt,
	}
	if g.SecondaryGatewayID.Valid {
		secIDStr := uuid.UUID(g.SecondaryGatewayID.Bytes).String()
		resp.SecondaryGatewayID = &secIDStr
	}
	if g.LastSeen.Valid {
		resp.LastHeartBeat = &g.LastSeen.Time
	}
	if g.EnrolledAt.Valid {
		resp.EnrolledAt = &g.EnrolledAt.Time
	}
	if g.RevokedAt.Valid {
		resp.RevokedAt = &g.RevokedAt.Time
	}
	return resp
}

func mapToConnectorResponseRow(g store.ListConnectorsWithGatewayNameByOrgRow, apps []store.App) ConnectorResponse {
	resp := ConnectorResponse{
		ID:           g.ID.String(),
		Name:         g.Name,
		OrgID:        g.OrgID.String(),
		GatewayID:    g.GatewayID.String(),
		GatewayName:  g.GatewayName,
		ActiveStream: g.ActiveStreams,
		OpenSock:     g.OpenSock,
		Status:       g.Status,
		CreatedAt:    g.CreatedAt,
	}
	if g.SecondaryGatewayID.Valid {
		secIDStr := uuid.UUID(g.SecondaryGatewayID.Bytes).String()
		resp.SecondaryGatewayID = &secIDStr
	}
	if g.SecondaryGatewayName.Valid {
		resp.SecondaryGatewayName = &g.SecondaryGatewayName.String
	}
	if g.LastSeen.Valid {
		resp.LastHeartBeat = &g.LastSeen.Time
	}
	if g.EnrolledAt.Valid {
		resp.EnrolledAt = &g.EnrolledAt.Time
	}
	if g.RevokedAt.Valid {
		resp.RevokedAt = &g.RevokedAt.Time
	}

	appSummaries := make([]ConnectorAppSummaryResponse, 0, len(apps))
	for _, app := range apps {
		appSummaries = append(appSummaries, ConnectorAppSummaryResponse{
			ID:           app.ID.String(),
			Name:         app.Name,
			Subdomain:    app.Subdomain,
			HealthStatus: app.HealthStatus,
		})
	}
	resp.Apps = appSummaries
	return resp
}

// CreateConnector creates a new connector.
func (s *Service) CreateConnector(ctx context.Context, name string, gatewayID uuid.UUID, secondaryGatewayID *uuid.UUID, OpenSock bool) (ConnectorResponse, *dto.AppError) {

	//Check if the Org is same as the Admin
	AdminOrgId := middleware.OrgIDFromCtx(ctx)

	AdminUUID, err := uuid.Parse(AdminOrgId)

	if err != nil {
		return ConnectorResponse{}, dto.NewUnauthorizedError("OrgID Error")
	}

	//Generate a 6 chars token and hash
	token := utils.GenerateRandomString(6)

	var secGwParam pgtype.UUID
	if secondaryGatewayID != nil {
		secGwParam = pgtype.UUID{Bytes: *secondaryGatewayID, Valid: true}
	}

	params := store.CreateConnectorParams{
		OrgID:              AdminUUID,
		Name:               name,
		GatewayID:          gatewayID,
		SecondaryGatewayID: secGwParam,
		TokenHash:          utils.HashToken(token),
		OpenSock:           OpenSock,
	}
	connector, err := s.repo.CreateConnector(ctx, params)
	if err != nil {
		return ConnectorResponse{}, dto.NewAppError(500, dto.CodeInternal, "Failed to create connector", err.Error())
	}

	//Send Updated Active Connectors list to Primary & Secondary Gateway
	s.SyncConnectorToGateway(ctx, connector.GatewayID)
	if connector.SecondaryGatewayID.Valid {
		s.SyncConnectorToGateway(ctx, uuid.UUID(connector.SecondaryGatewayID.Bytes))
	}

	return mapToConnectorResponse(connector, token), nil
}

func (s *Service) SyncConnectorToGateway(ctx context.Context, gatewayID uuid.UUID) {
	//Get all Connectors after creation
	connectors, _ := s.repo.ListActiveConnectorsByGateway(ctx, gatewayID)

	var connectorsInfo []*proto.ConnectorInfo
	for _, connector := range connectors {
		connectorsInfo = append(connectorsInfo, &proto.ConnectorInfo{
			Id:     connector.ID.String(),
			Status: connector.Status,
		})
	}

	payload := events.CommandJob{
		Type:          events.CmdConnectorSync,
		GatewayID:     gatewayID.String(),
		ConnectorInfo: connectorsInfo,
	}

	_, _ = s.eventRepo.CreateEvent(ctx, payload)
	s.dispatcher.Wakeup(gatewayID.String())
}

// ListConnectorsByOrg lists all connectors for a given organization.
func (s *Service) ListConnectorsByOrg(ctx context.Context, orgID uuid.UUID) ([]ConnectorResponse, *dto.AppError) {
	connectors, err := s.repo.ListConnectorsWithGatewayNameByOrg(ctx, orgID)
	if err != nil {
		return nil, dto.NewAppError(500, dto.CodeInternal, "Failed to list connectors", err.Error())
	}

	var responses []ConnectorResponse
	for _, g := range connectors {
		apps, _ := s.repo.GetConnectorApps(ctx, pgtype.UUID{Bytes: g.ID, Valid: true})
		responses = append(responses, mapToConnectorResponseRow(g, apps))
	}
	if responses == nil {
		responses = []ConnectorResponse{}
	}
	return responses, nil
}

// ReEnrollConnector re-create a connector.
func (s *Service) ReCreateConnector(ctx context.Context, id uuid.UUID, name string, gatewayId uuid.UUID, secondaryGatewayID *uuid.UUID, OpenSock bool) (ConnectorResponse, *dto.AppError) {

	connectorRes, err := s.repo.GetConnectorByID(ctx, id)

	if err != nil {
		return ConnectorResponse{}, dto.NewNotFoundError("Not Found")
	}

	//Check if the Org is same as the Admin
	AdminOrgId := middleware.OrgIDFromCtx(ctx)
	if AdminOrgId != connectorRes.OrgID.String() {
		return ConnectorResponse{}, dto.NewUnauthorizedError("OrgID Error")
	}

	//Generate a 6 chars token and hash
	token := utils.GenerateRandomString(6)

	var secGwParam pgtype.UUID
	if secondaryGatewayID != nil {
		secGwParam = pgtype.UUID{Bytes: *secondaryGatewayID, Valid: true}
	}

	params := store.ReCreateConnectorParams{
		ID:                 id,
		GatewayID:          gatewayId,
		SecondaryGatewayID: secGwParam,
		Name:               name, // Assuming name can be updated during re-enrollment
		TokenHash:          utils.HashToken(token),
		OpenSock:           OpenSock,
	}
	connector, err := s.repo.ReCreateConnector(ctx, params)
	if err != nil {
		return ConnectorResponse{}, dto.NewAppError(500, dto.CodeInternal, "Failed to re-enroll connector", err.Error())
	}

	//Send Updated Active Connectors list to Primary & Secondary Gateway
	s.SyncConnectorToGateway(ctx, connector.GatewayID)
	if connector.SecondaryGatewayID.Valid {
		s.SyncConnectorToGateway(ctx, uuid.UUID(connector.SecondaryGatewayID.Bytes))
	}

	return mapToConnectorResponse(connector, token), nil
}

// EnrollConnector enrolls a connector using a token hash and CSR.
func (s *Service) EnrollConnector(ctx context.Context, token, csr string, signer pki.CASigner) (gen.ConnectorEnrollResponse, *dto.AppError) {
	// Get the Connector by using the token
	conn, err := s.repo.GetConnectorByTokenHash(ctx, utils.HashToken(token))
	if err != nil {
		return gen.ConnectorEnrollResponse{}, dto.NewNotFoundError("Gateway Not Found")
	}

	// Parse CSR
	certReq, err := pki_utils.ParseCSR([]byte(csr))
	if err != nil {
		return gen.ConnectorEnrollResponse{}, dto.NewBadRequestError("Not Valid CSR: " + err.Error())
	}

	//Validate the CSR fields
	if len(certReq.Subject.OrganizationalUnit) == 0 || certReq.Subject.OrganizationalUnit[0] != "Connector" {
		return gen.ConnectorEnrollResponse{}, dto.NewUnauthorizedError("Not a Connector")
	}

	// Sign the cert
	connCRT, err := signer.IssueCert(certReq, 90*24*time.Hour, conn.ID.String(), "ashrix.io")
	if err != nil {
		return gen.ConnectorEnrollResponse{}, dto.NewBadRequestError("Error in signing CSR: " + err.Error())
	}

	// Get active intermediate CA certificate from DB to retrieve its ID
	caCertRecord, err := s.pkiRepo.GetActiveCACert(ctx, store.GetActiveCACertParams{
		Name: "Ashrix Intermediate CA",
		Type: "intermediate",
	})
	if err != nil {
		return gen.ConnectorEnrollResponse{}, dto.NewAppError(500, dto.CodeInternal, "Failed to retrieve active Intermediate CA", err.Error())
	}

	// Register component cert
	_, err = s.pkiRepo.CreateComponentCert(ctx, store.RegisterCompCertParams{
		OrgID:         pgtype.UUID{Valid: true, Bytes: conn.OrgID},
		ComponentType: "connector",
		ComponentID:   pgtype.UUID{Valid: true, Bytes: conn.ID},
		CaID:          caCertRecord.ID,
		CertPem:       string(pki_utils.MarshalCert(connCRT)),
		SerialNumber:  connCRT.SerialNumber.String(),
		Subject:       connCRT.Subject.CommonName,
		San:           connCRT.DNSNames,
		IssuedAt:      connCRT.NotBefore,
		ExpiresAt:     connCRT.NotAfter,
		RotationOf:    pgtype.UUID{Valid: false},
	})
	if err != nil {
		return gen.ConnectorEnrollResponse{}, dto.NewAppError(500, dto.CodeInternal, "Failed to register gateway certificate", err.Error())
	}

	// Update conn status to healthy/enrolled
	_, err = s.repo.EnrollConnectorUsingTokenHash(ctx, utils.HashToken(token))
	if err != nil {
		return gen.ConnectorEnrollResponse{}, dto.NewAppError(500, dto.CodeInternal, "Failed to enroll gateway", err.Error())
	}

	//Send Updated Active Connectors list to Gateway
	s.SyncConnectorToGateway(ctx, conn.GatewayID)

	return gen.ConnectorEnrollResponse{
		ConnectorId: conn.ID.String(),
		Certificate: string(pki_utils.MarshalCert(connCRT)),
		TrustBundle: string(signer.TrustBundle()),
		OpenSock:    conn.OpenSock,
		ExpiresAt:   timestamppb.New(connCRT.NotAfter),
	}, nil
}

func (s *Service) RenewConnectorCert(ctx context.Context, connectorID uuid.UUID, signature string, csr string, timestamp int64, signer pki.CASigner) (gen.ConnectorRenewCertResponse, *dto.AppError) {
	//Check timestamp is within 60 secsy
	reqTime := time.Unix(timestamp, 0)
	if time.Since(reqTime) > 60*time.Second || time.Until(reqTime) > 60*time.Second {
		return gen.ConnectorRenewCertResponse{}, dto.NewUnauthorizedError("Timestamp out of acceptable window (+/- 60s)")
	}

	// get connector
	connector, err := s.repo.GetActiveConnectorByID(ctx, connectorID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return gen.ConnectorRenewCertResponse{}, dto.NewNotFoundError("Gateway Not Found")
		}
		return gen.ConnectorRenewCertResponse{}, dto.NewAppError(500, dto.CodeInternal, "Failed to retrieve connector", err.Error())
	}

	// get active cert
	activeCert, err := s.pkiRepo.GetActiveComponentCert(ctx, store.GetActiveComponentCertParams{
		ComponentType: "connector",
		ComponentID:   pgtype.UUID{Valid: true, Bytes: connector.ID},
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return gen.ConnectorRenewCertResponse{}, dto.NewNotFoundError("Active Component Certificate Not Found")
		}
		return gen.ConnectorRenewCertResponse{}, dto.NewAppError(500, dto.CodeInternal, "Failed to retrieve active component certificate", err.Error())
	}

	certPem := []byte(activeCert.CertPem)
	pubkey, err := pki_utils.ParsePublicKeyFromCert(certPem)
	if err != nil {
		return gen.ConnectorRenewCertResponse{}, dto.NewAppError(500, dto.CodeInternal, "Failed to parse active component certificate", err.Error())
	}

	// verify signature
	verified := pki_utils.VerifyPossessionProof(pubkey, []byte(csr), connectorID.String(), timestamp, signature)
	if !verified {
		return gen.ConnectorRenewCertResponse{}, dto.NewUnauthorizedError("Signature verification failed")
	}

	//Check timestamp is within 60 secsy
	reqTime = time.Unix(timestamp, 0)
	if time.Since(reqTime) > 60*time.Second || time.Until(reqTime) > 60*time.Second {
		return gen.ConnectorRenewCertResponse{}, dto.NewUnauthorizedError("Timestamp out of acceptable window (+/- 60s)")
	}

	// Issue new cert
	// get active intermediate CA
	caCertRecord, err := s.pkiRepo.GetActiveCACert(ctx, store.GetActiveCACertParams{
		Name: "Ashrix Intermediate CA",
		Type: "intermediate",
	})
	if err != nil {
		return gen.ConnectorRenewCertResponse{}, dto.NewAppError(500, dto.CodeInternal, "Failed to retrieve active Intermediate CA", err.Error())
	}

	//Revoke the current cert of the gateway
	_, err = s.pkiRepo.RevokeComponentCert(ctx, store.RevokeCompCertParams{
		ComponentID:   pgtype.UUID{Valid: true, Bytes: connectorID},
		ComponentType: "connector",
		RevokeReason:  pgtype.Text{String: "superseded", Valid: true},
	})

	if err != nil {
		return gen.ConnectorRenewCertResponse{}, dto.NewAppError(500, dto.CodeInternal, "Failed to revoke gateway certificate", err.Error())
	}

	//Parse CSR
	certReq, err := pki_utils.ParseCSR([]byte(csr))
	if err != nil {
		return gen.ConnectorRenewCertResponse{}, dto.NewBadRequestError("Not Valid CSR: " + err.Error())
	}

	//Validate the csr fields
	if len(certReq.Subject.OrganizationalUnit) == 0 || certReq.Subject.OrganizationalUnit[0] != "Connector" {
		return gen.ConnectorRenewCertResponse{}, dto.NewUnauthorizedError("Not a connector")
	}

	// Issue new cert
	connCRT, err := signer.IssueCert(certReq, 90*24*time.Hour, connectorID.String(), "ashrix.io")
	if err != nil {
		return gen.ConnectorRenewCertResponse{}, dto.NewBadRequestError("Error in signing CSR: " + err.Error())
	}

	//Register the cert
	_, err = s.pkiRepo.CreateComponentCert(ctx, store.RegisterCompCertParams{
		OrgID:         pgtype.UUID{Valid: true, Bytes: connector.OrgID},
		ComponentID:   pgtype.UUID{Valid: true, Bytes: connectorID},
		ComponentType: "connector",
		CaID:          caCertRecord.ID,
		CertPem:       string(pki_utils.MarshalCert(connCRT)),
		SerialNumber:  connCRT.SerialNumber.String(),
		Subject:       connCRT.Subject.CommonName,
		San:           connCRT.DNSNames,
		IssuedAt:      connCRT.NotBefore,
		ExpiresAt:     connCRT.NotAfter,
		RotationOf:    pgtype.UUID{Valid: false},
	})
	if err != nil {
		return gen.ConnectorRenewCertResponse{}, dto.NewAppError(500, dto.CodeInternal, "Failed to register conn certificate", err.Error())
	}

	//Send Updated Active Connectors list to Gateway
	s.SyncConnectorToGateway(ctx, connector.GatewayID)

	return gen.ConnectorRenewCertResponse{
		Certificate: string(pki_utils.MarshalCert(connCRT)),
		TrustBundle: string(signer.TrustBundle()),
		OpenSock:    connector.OpenSock,
		ExpiresAt:   timestamppb.New(connCRT.NotAfter),
	}, nil
}

// RevokeConnectorCert revokes a conn certificate.
// It assumes componentID is the conn's ID.
func (s *Service) RevokeConnectorCert(ctx context.Context, connectorId uuid.UUID, revokeReason string) (ConnectorResponse, *dto.AppError) {
	// Retrieve the admin's OrgID from context
	adminOrgIDStr := middleware.OrgIDFromCtx(ctx)

	// Validate the revocation reason against DB check constraints
	switch revokeReason {
	case "keyCompromise", "superseded", "cessationOfOperation", "affiliationChanged":
		// valid reason
	default:
		return ConnectorResponse{}, dto.NewBadRequestError("Invalid revocation reason")
	}

	// Fetch the connector first to verify it exists
	connector, err := s.repo.GetConnectorByID(ctx, connectorId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ConnectorResponse{}, dto.NewNotFoundError("Connector Not Found")
		}
		return ConnectorResponse{}, dto.NewAppError(500, dto.CodeInternal, "Failed to retrieve Connector", err.Error())
	}

	// Verify that the connector belongs to the admin's organization
	if adminOrgIDStr != connector.OrgID.String() {
		return ConnectorResponse{}, dto.NewUnauthorizedError("OrgID Error")
	}

	// Fetch active component certificate
	activeCert, err := s.pkiRepo.GetActiveComponentCert(ctx, store.GetActiveComponentCertParams{
		ComponentType: "connector",
		ComponentID:   pgtype.UUID{Valid: true, Bytes: connectorId},
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// No active cert to revoke, but we can return the connector response as success
			return mapToConnectorResponse(connector, ""), nil
		}
		return ConnectorResponse{}, dto.NewAppError(500, dto.CodeInternal, "Failed to retrieve active component certificate", err.Error())
	}

	// Revoke component cert
	params := store.RevokeCompCertParams{
		ComponentID:   pgtype.UUID{Valid: true, Bytes: connectorId},
		ComponentType: "connector",
		RevokeReason:  pgtype.Text{String: revokeReason, Valid: true},
	}
	_, err = s.pkiRepo.RevokeComponentCert(ctx, params)
	if err != nil {
		return ConnectorResponse{}, dto.NewAppError(500, dto.CodeInternal, "Failed to revoke component certificate", err.Error())
	}

	// Create CRL Entry
	_, err = s.pkiRepo.CreateCRLEntry(ctx, store.CreateCRLEntryParams{
		CertID:       activeCert.ID,
		SerialNumber: activeCert.SerialNumber,
		Reason:       revokeReason,
	})
	if err != nil {
		return ConnectorResponse{}, dto.NewAppError(500, dto.CodeInternal, "Failed to create CRL entry", err.Error())
	}

	//Get all the active CRLs
	activeCRLs, _ := s.pkiRepo.GetCRLEntry(ctx)

	var revokedSerials []string
	for _, crl := range activeCRLs {
		revokedSerials = append(revokedSerials, crl.SerialNumber)
	}

	// Dispatch commands to Primary Gateway
	targetGateways := []string{connector.GatewayID.String()}
	if connector.SecondaryGatewayID.Valid {
		targetGateways = append(targetGateways, uuid.UUID(connector.SecondaryGatewayID.Bytes).String())
	}

	for _, gwID := range targetGateways {
		payload := events.CommandJob{
			Type:        events.CmdRevokeConnectorCert,
			GatewayID:   gwID,
			ConnectorID: connectorId.String(),
		}
		_, _ = s.eventRepo.CreateEvent(ctx, payload)

		payload2 := events.CommandJob{
			Type:                 events.CmdCrlSync,
			GatewayID:            gwID,
			RevokedSerialNumbers: revokedSerials,
		}
		_, _ = s.eventRepo.CreateEvent(ctx, payload2)
		s.dispatcher.Wakeup(gwID)
		s.SyncConnectorToGateway(ctx, uuid.MustParse(gwID))
	}

	// Fetch updated connector status
	updatedConnector, err := s.repo.GetConnectorByID(ctx, connectorId)
	if err != nil {
		return ConnectorResponse{}, dto.NewAppError(500, dto.CodeInternal, "Failed to retrieve gateway", err.Error())
	}

	return mapToConnectorResponse(updatedConnector, ""), nil
}

// RevokeConnector revokes an entire Connector.
func (s *Service) RevokeConnector(ctx context.Context, id uuid.UUID) (ConnectorResponse, *dto.AppError) {
	// Retrieve the admin's OrgID from context
	adminOrgIDStr := middleware.OrgIDFromCtx(ctx)
	adminOrgID, parseErr := uuid.Parse(adminOrgIDStr)
	if parseErr != nil {
		return ConnectorResponse{}, dto.NewBadRequestError("Invalid Organization ID format")
	}

	// Fetch the Connector to verify it exists
	Connector, err := s.repo.GetConnectorByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ConnectorResponse{}, dto.NewNotFoundError("Connector Not Found")
		}
		return ConnectorResponse{}, dto.NewAppError(500, dto.CodeInternal, "Failed to retrieve Connector", err.Error())
	}

	// Verify that the Connector belongs to the admin's organization
	if adminOrgID != Connector.OrgID {
		return ConnectorResponse{}, dto.NewUnauthorizedError("OrgID Error")
	}

	// Mark the Connector as revoked (offline) in the database
	params := store.RevokeConnectorParams{
		ID:        Connector.ID,
		GatewayID: Connector.GatewayID,
	}
	revokedConnector, err := s.repo.RevokeConnector(ctx, params)
	if err != nil {
		return ConnectorResponse{}, dto.NewAppError(500, dto.CodeInternal, "Failed to revoke Connector record", err.Error())
	}

	// Revoke the active component certificate if it exists
	activeCert, err := s.pkiRepo.GetActiveComponentCert(ctx, store.GetActiveComponentCertParams{
		ComponentType: "connector",
		ComponentID:   pgtype.UUID{Valid: true, Bytes: id},
	})
	if err == nil {
		// Cert found, revoke it
		_, err = s.pkiRepo.RevokeComponentCert(ctx, store.RevokeCompCertParams{
			ComponentID:   pgtype.UUID{Valid: true, Bytes: id},
			ComponentType: "connector",
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

	//Get all the active CRLs
	activeCRLs, _ := s.pkiRepo.GetCRLEntry(ctx)

	var revokedSerials []string
	for _, crl := range activeCRLs {
		revokedSerials = append(revokedSerials, crl.SerialNumber)
	}

	targetGateways := []string{Connector.GatewayID.String()}
	if Connector.SecondaryGatewayID.Valid {
		targetGateways = append(targetGateways, uuid.UUID(Connector.SecondaryGatewayID.Bytes).String())
	}

	for _, gwID := range targetGateways {
		payload := events.CommandJob{
			Type:        events.CmdRevokeConnector,
			GatewayID:   gwID,
			ConnectorID: Connector.ID.String(),
		}
		_, _ = s.eventRepo.CreateEvent(ctx, payload)

		payload2 := events.CommandJob{
			Type:                 events.CmdCrlSync,
			GatewayID:            gwID,
			RevokedSerialNumbers: revokedSerials,
		}
		_, _ = s.eventRepo.CreateEvent(ctx, payload2)
		s.dispatcher.Wakeup(gwID)
		s.SyncConnectorToGateway(ctx, uuid.MustParse(gwID))
	}

	return mapToConnectorResponse(revokedConnector, ""), nil
}

// Send Rotate Command to Gateway for Connector
func (s *Service) SendRotateConnectorCmd(ctx context.Context, connectorId uuid.UUID) (ConnectorResponse, *dto.AppError) {
	// Retrieve the admin's OrgID from context
	adminOrgIDStr := middleware.OrgIDFromCtx(ctx)
	adminOrgID, parseErr := uuid.Parse(adminOrgIDStr)
	if parseErr != nil {
		return ConnectorResponse{}, dto.NewBadRequestError("Invalid Organization ID format")
	}

	// Fetch the Connector to verify it exists
	Connector, err := s.repo.GetConnectorByID(ctx, connectorId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ConnectorResponse{}, dto.NewNotFoundError("Connector Not Found")
		}
		return ConnectorResponse{}, dto.NewAppError(500, dto.CodeInternal, "Failed to retrieve Connector", err.Error())
	}

	// Verify that the Connector belongs to the admin's organization
	if adminOrgID != Connector.OrgID {
		return ConnectorResponse{}, dto.NewUnauthorizedError("OrgID Error")
	}

	targetGateways := []string{Connector.GatewayID.String()}
	if Connector.SecondaryGatewayID.Valid {
		targetGateways = append(targetGateways, uuid.UUID(Connector.SecondaryGatewayID.Bytes).String())
	}

	for _, gwID := range targetGateways {
		payload := events.CommandJob{
			Type:        events.CmdRotateConnectorCert,
			GatewayID:   gwID,
			ConnectorID: Connector.ID.String(),
		}
		_, _ = s.eventRepo.CreateEvent(ctx, payload)
		s.dispatcher.Wakeup(gwID)
	}

	return mapToConnectorResponse(Connector, ""), nil
}

func (s *Service) GetConnectorStatus(ctx context.Context, connectorID uuid.UUID, canonicalString, signature string) (*gen.ConnectorStatusResponse, *dto.AppError) {

	ConnectorRow, err := s.repo.GetActiveConnectorByID(ctx, connectorID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &gen.ConnectorStatusResponse{}, dto.NewNotFoundError("Connector Not Found")
		}
		return &gen.ConnectorStatusResponse{}, dto.NewAppError(500, dto.CodeInternal, "Failed to retrieve Connector", err.Error())
	}

	GatewayRow, err := s.gatewayRepo.GetActiveGatewayByID(ctx, ConnectorRow.GatewayID)

	if err != nil {
		return &gen.ConnectorStatusResponse{}, dto.NewAppError(500, dto.CodeInternal, "Failed to retrieve Primary Gateway", err.Error())
	}

	//Get the active Cert for the connector
	activeCert, err := s.pkiRepo.GetActiveComponentCert(ctx, store.GetActiveComponentCertParams{
		ComponentType: "connector",
		ComponentID:   pgtype.UUID{Valid: true, Bytes: connectorID},
	})

	if err != nil {
		return &gen.ConnectorStatusResponse{}, dto.NewNotFoundError("Cert Not Found")
	}

	block, _ := pem.Decode([]byte(activeCert.CertPem))
	if block == nil {
		return &gen.ConnectorStatusResponse{}, dto.NewNotFoundError("Cert Not Found")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return &gen.ConnectorStatusResponse{}, dto.NewBadRequestError("Error Parsing Cert")
	}
	pub, ok := cert.PublicKey.(*ecdsa.PublicKey)
	if !ok {
		return &gen.ConnectorStatusResponse{}, dto.NewBadRequestError("Error Parsing Key")
	}

	//Check the Signature and canonicalString
	if err = utils.VerifySignature(pub, canonicalString, signature); err != nil {
		return &gen.ConnectorStatusResponse{}, dto.NewUnauthorizedError("Invalid Signature")
	}

	//Get the apps of the connector
	apps, err := s.repo.GetConnectorApps(ctx, pgtype.UUID{Valid: true, Bytes: connectorID})

	var appsResp []*gen.ConnectorApps

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			appsResp = []*gen.ConnectorApps{}
		}
		return &gen.ConnectorStatusResponse{}, dto.NewNotFoundError("Connector Not Found")
	}

	if len(apps) > 0 {
		for _, g := range apps {
			app := &gen.ConnectorApps{
				Id:                    g.ID.String(),
				Name:                  g.Name,
				Subdomain:             g.Subdomain,
				Upstream:              g.Upstream,
				Protocol:              g.Protocol,
				IsPublic:              g.IsPublic,
				SockPass:              g.SockPass,
				CheckHealth:           g.CheckHealth,
				CheckInterval:         g.CheckInterval,
				HealthEndpoint:        g.HealthEndpoint.String,
				HealthStatus:          g.HealthStatus,
				EnableSecurityHeaders: g.EnableSecurityHeaders,
			}
			appsResp = append(appsResp, app)
		}
	}

	resp := &gen.ConnectorStatusResponse{
		ConnectorId: ConnectorRow.ID.String(),
		GatewayId:   ConnectorRow.GatewayID.String(),
		GatewayUrl:  GatewayRow.PublicUrl,
		GatewayIp:   GatewayRow.IpAddress,
		OpenSock:    ConnectorRow.OpenSock,
		TenantId:    ConnectorRow.OrgID.String(),
		GrpcPort:    GatewayRow.GrpcPort.String,
		QuicPort:    GatewayRow.QuicPort.String,
		Apps:        appsResp,
	}

	if ConnectorRow.SecondaryGatewayID.Valid {
		secGWUUID := uuid.UUID(ConnectorRow.SecondaryGatewayID.Bytes)
		if secGW, err := s.gatewayRepo.GetActiveGatewayByID(ctx, secGWUUID); err == nil {
			resp.SecondaryGatewayId = secGW.ID.String()
			resp.SecondaryGatewayUrl = secGW.PublicUrl
			resp.SecondaryGatewayIp = secGW.IpAddress
			resp.SecondaryGrpcPort = secGW.GrpcPort.String
			resp.SecondaryHttpsPort = secGW.HttpsPort.String
			resp.SecondaryQuicPort = secGW.QuicPort.String
		}
	}

	return resp, nil
}
