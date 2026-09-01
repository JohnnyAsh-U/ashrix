package cp_grpc

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/app"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/connector"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/database/store"
	gatewayevents "github.com/JohnnyAsh-U/ashrix-api/internal/cp/events"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/gateway"
	pkica "github.com/JohnnyAsh-U/ashrix-api/internal/cp/pki_ca"
	// "github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/cp_grpc/dispatcher"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/cp_grpc/registry"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/policy"
	proto "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type cpServer struct {
	proto.UnimplementedControlPlaneServiceServer

	registry      *registry.GatewayRegistry
	redisClient   *redis.Client
	distributor   *policy.PolicyDistributor
	policyStore   policy.Repository
	gatewayRepo   gateway.Repository
	eventRepo     gatewayevents.Repository
	PkiCARepo     pkica.Repository
	connectorRepo connector.Repository
	appRepo       app.Repository
	// dbQueries   *store.Queries
	log *slog.Logger
}

// Connect handles the bidirectional stream from a gateway
func (s *cpServer) Connect(stream proto.ControlPlaneService_ConnectServer) error {
	// 1. Extract peer cert and gateway ID
	peerInfo, ok := peer.FromContext(stream.Context())
	if !ok {
		return status.Error(codes.Unauthenticated, "no peer info")
	}
	tlsInfo, ok := peerInfo.AuthInfo.(credentials.TLSInfo)
	if !ok {
		return status.Error(codes.Unauthenticated, "no TLS Info")
	}
	if len(tlsInfo.State.PeerCertificates) == 0 {
		return status.Error(codes.Unauthenticated, "no peer certificates")
	}
	cert := tlsInfo.State.PeerCertificates[0]
	certSerial := cert.SerialNumber.String()
	gatewayIDInCert := cert.Subject.CommonName

	gatewayUUID, err := uuid.Parse(gatewayIDInCert)
	if err != nil {
		return status.Error(codes.InvalidArgument, "invalid gateway ID in cert")
	}

	// 2. Check if gateway is revoked
	gw, err := s.gatewayRepo.GetActiveGatewayByID(stream.Context(), gatewayUUID)
	if err != nil {
		return status.Error(codes.Unauthenticated, "gateway is not registered or revoked")
	}
	if !gw.IsActive {
		return status.Error(codes.Unauthenticated, "gateway is not active")
	}

	// 3. Check if cert is in CRL entries
	_, err = s.PkiCARepo.GetCRLEntryBySerial(stream.Context(), certSerial)
	if err == nil {
		return status.Error(codes.Unauthenticated, "gateway certificate is revoked (CRL)")
	}

	// 4. Check if component certificate is active and not revoked
	activeCert, err := s.PkiCARepo.GetActiveComponentCert(stream.Context(), store.GetActiveComponentCertParams{
		ComponentType: "gateway",
		ComponentID:   pgtype.UUID{Bytes: gatewayUUID, Valid: true},
	})
	if err != nil {
		return status.Error(codes.Unauthenticated, "active gateway certificate not found")
	}
	if activeCert.RevokedAt.Valid {
		return status.Error(codes.Unauthenticated, "gateway certificate is revoked")
	}

	// 5. Check if another connection for this gateway already exists
	if _, exists := s.registry.GetConnection(gatewayIDInCert); exists {
		return status.Error(codes.AlreadyExists, "another connection already exists for this gateway")
	}

	ctx, cancel := context.WithCancel(stream.Context())
	defer cancel()

	connection := registry.NewGatewayConn(
		ctx,
		cancel,
		stream,
		gatewayUUID.String(),
		gw.OrgID.String(),
	)

	// Register the connection
	s.registry.Register(connection)
	defer s.registry.Unregister(connection.GatewayID)

	// ==================================================
	// 5. Reconcile durable gateway events
	// ==================================================

	if err := s.reconcileGateway(
		ctx,
		connection,
	); err != nil {

		s.log.Error(
			"gateway reconciliation failed",
			slog.String("gateway_id", gatewayIDInCert),
			slog.String("error", err.Error()),
		)
		return status.Error(
			codes.Internal,
			"gateway reconciliation failed",
		)
	}

	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			log.Fatal("Gateway Disconnected Cleanly")
			return nil
		}
		if err != nil {
			log.Println(err)
			return nil
		}
		connection.LastSeen = time.Now()

		switch msg.Payload.(type) {

		case *proto.GatewayEnvelope_Hello:
			currentPolicy := msg.GetHello().PolicyVersion
			tenantID := msg.GetHello().TenantId
			tenantUUID, _ := uuid.Parse(tenantID)
			log.Printf("gateway connected: %s (policy_version=%d)",
				gatewayIDInCert, msg.GetHello().PolicyVersion)

			// If the gateway is behind on policy, push the latest immediately
			if currentPolicy < s.distributor.LatestVersion(tenantUUID) {
				go s.distributor.PushToGateway(ctx, connection, tenantUUID)
			}
			s.handleHello(ctx, connection, msg.GetHello())

		case *proto.GatewayEnvelope_Heartbeat:
			s.handleHeartbeat(ctx, connection, msg.GetHeartbeat())

		case *proto.GatewayEnvelope_CmdAck:
			if err := s.handleCommandAck(ctx, connection, msg.GetCmdAck()); err != nil {
				s.log.Error(
					"Command ACK error",
					slog.String("gateway_id", connection.GatewayID),
					slog.String("error", err.Error()),
				)
				continue
			}

		default:
			log.Printf("Unknown message")
		}
	}
}

func (s *cpServer) handleCommandAck(
	ctx context.Context,
	conn *registry.GatewayConn,
	ack *proto.CommandAck,
) error {

	if ack.GatewayId != conn.GatewayID {
		return status.Error(
			codes.PermissionDenied,
			"gateway ACK identity mismatch",
		)
	}

	seq := ack.ProcessedThroughSeq

	if seq < 0 {
		return status.Error(
			codes.InvalidArgument,
			"invalid ACK sequence",
		)
	}

	gatewayID, err :=
		uuid.Parse(conn.GatewayID)

	if err != nil {
		return err
	}

	// --------------------------------------------
	// Verify ACK cannot jump beyond known events
	// --------------------------------------------

	latest, err :=
		s.eventRepo.GetLatestGatewayEventSeq(
			ctx,
			gatewayID,
		)

	if err != nil {
		return err
	}

	if seq > latest {
		return status.Errorf(
			codes.InvalidArgument,
			"ACK %d exceeds latest event %d",
			seq,
			latest,
		)
	}

	// --------------------------------------------
	// Monotonic database update
	// --------------------------------------------

	if err :=
		s.eventRepo.AckGatewayEvents(
			ctx,
			store.AckGatewayEventsParams{
				GatewayID:    gatewayID,
				LastAckedSeq: seq,
			},
		); err != nil {

		return fmt.Errorf(
			"persist gateway ACK: %w",
			err,
		)
	}

	// Wake any active delivery waiter.
	conn.ResolveThrough(seq, nil)

	return nil
}

func (s *cpServer) handleHello(ctx context.Context, conn *registry.GatewayConn, hello *proto.HelloMessage) error {

	if hello.GatewayId != conn.GatewayID {
		return status.Error(
			codes.Unauthenticated,
			"gateway identity mismatch",
		)
	}

	gatewayID, err := uuid.Parse(conn.GatewayID)
	if err != nil {
		return err
	}


	_, err = s.gatewayRepo.UpdateGatewayBinaryVersion(ctx, store.UpdateGatewayBinaryVersionParams{
		ID: gatewayID,
		Version: pgtype.Text{
			Valid:  true,
			String: hello.BinaryVersion,
		},
	},
	)
	if err != nil {
		s.log.Warn(
			"failed to update gateway binary version",
			"gateway_id",
			conn.GatewayID,
			"error",
			err,
		)
	}

	//Get the authorized connectors
	connectors, err := s.connectorRepo.ListActiveConnectorsByGateway(ctx, gatewayID)
	if err != nil {
		s.log.Warn("Failed to get Connectors")
	}
	var connectorsInfo []*proto.ConnectorInfo
	for _, connector := range connectors {
		connectorsInfo = append(connectorsInfo, &proto.ConnectorInfo{
			Id:     connector.ID.String(),
			Status: connector.Status,
		})
	}

	return conn.Send(
		&proto.CPEnvelope{
			Payload: &proto.CPEnvelope_HelloAck{
				HelloAck: &proto.HelloAck{
					ServerVersion: "1.0.0",
					ServerTime:    timestamppb.Now(),
					Connectors: connectorsInfo,
				},
			},
		},
	)
}

func (s *cpServer) handleHeartbeat(
	ctx context.Context,
	conn *registry.GatewayConn,
	heartbeat *proto.HeartbeatMessage,
) error {

	log.Println("HeartBeat", heartbeat.Seq)

	if heartbeat.GatewayId != conn.GatewayID {
		return status.Error(
			codes.PermissionDenied,
			"heartbeat gateway mismatch",
		)
	}

	gatewayID, err :=
		uuid.Parse(conn.GatewayID)

	if err != nil {
		return err
	}

	_, err =
		s.gatewayRepo.UpdateGatewayHeartBeat(
			ctx,
			gatewayID,
		)
	if err != nil {
		return err
	}

	for _, connStat := range heartbeat.ConnectorStatus {
		cID, err := uuid.Parse(connStat.ConnectorId)
		if err != nil {
			s.log.Warn("invalid connector ID in heartbeat telemetry", "connector_id", connStat.ConnectorId, "error", err)
			continue
		}
		_, err = s.connectorRepo.UpdateConnectorStatus(ctx, store.UpdateConnectorStatusParams{
			ID:     cID,
			Status: connStat.Status,
			ActiveStreams: heartbeat.ActiveSessions,
		})
		if err != nil {
			s.log.Warn("failed to update connector status in DB", "connector_id", connStat.ConnectorId, "error", err)
		}
	}

	for _, appHealth := range heartbeat.AppHealth {
		appUUID, err := uuid.Parse(appHealth.AppId)
		if err != nil {
			s.log.Warn("invalid app ID in heartbeat telemetry", "app_id", appHealth.AppId, "error", err)
			continue
		}
		var lastSeen pgtype.Timestamptz
		if appHealth.LastSeen != nil {
			lastSeen = pgtype.Timestamptz{Time: appHealth.LastSeen.AsTime(), Valid: true}
		} else {
			lastSeen = pgtype.Timestamptz{Time: time.Now(), Valid: true}
		}
		_, err = s.appRepo.UpdateAppHealthStatus(ctx, store.UpdateAppHealthStatusParams{
			ID:           appUUID,
			HealthStatus: appHealth.HealthStatus,
			LastSeen:     lastSeen,
		})
		if err != nil {
			s.log.Warn("failed to update app health status in DB", "app_id", appHealth.AppId, "error", err)
		}
	}

	return nil
}

func (s *cpServer) reconcileGateway(
	ctx context.Context,
	conn *registry.GatewayConn,
) error {

	gatewayID, err :=
		uuid.Parse(conn.GatewayID)

	if err != nil {
		return fmt.Errorf(
			"invalid gateway ID: %w",
			err,
		)
	}

	// --------------------------------------------
	// 1. Get authoritative cursor from CP
	// --------------------------------------------

	lastAck, err := s.eventRepo.GetLastAckedSeqForGateway(ctx, gatewayID)

	if err != nil {

		s.log.Error(
			"failed to get gateway event ACK",
			slog.String(
				"gateway_id",
				conn.GatewayID,
			),
			slog.String(
				"error",
				err.Error(),
			),
		)

		return err
	}

	// --------------------------------------------
	// 2. Get every event after cursor
	// --------------------------------------------

	events, err := s.eventRepo.ListGatewayEventsAfter(ctx, store.ListGatewayEventsAfterParams{GatewayID: gatewayID, Seq: lastAck})

	if err != nil {
		return fmt.Errorf("load gateway events: %w", err)
	}

	if len(events) == 0 {
		return nil
	}

	// --------------------------------------------
	// 3. Compact complete snapshots
	// --------------------------------------------

	events = s.eventRepo.CompactGatewayEvents(events)

	// --------------------------------------------
	// 4. Send in original sequence order
	// --------------------------------------------

	for _, event := range events {
		cmd, err := gatewayevents.GatewayEventToCmd(event)

		if err != nil {
			return fmt.Errorf(
				"convert event %d: %w",
				event.Seq,
				err,
			)
		}

		cmd.Seq = event.Seq
		cmd.EventId = event.EventID.String()

		envelope := &proto.CPEnvelope{
			SentAt: timestamppb.Now(),
			Payload: &proto.CPEnvelope_Cmd{
				Cmd: cmd,
			},
		}

		if err := conn.Send(envelope); err != nil {
			return fmt.Errorf(
				"send event %d: %w",
				event.Seq,
				err,
			)
		}
	}

	return nil
}

func (t *cpServer) ExchangeToken(ctx context.Context, req *proto.ExchangeTokenRequest) (*proto.ExchangeTokenResponse, error) {
	if req.TokenHash == "" || req.GatewayName == "" {
		return nil, status.Error(codes.InvalidArgument, "token_hash required")
	}
	key := fmt.Sprintf("session:%s:%s", req.GatewayName, req.TokenHash)

	data, err := t.redisClient.GetDel(ctx, key).Result()
	if err == redis.Nil {
		t.log.Warn("Token not found or already consumed", slog.Any("err", err))
		return &proto.ExchangeTokenResponse{Valid: false, ErrorMessage: "Invalid or Expired token"}, nil
	}

	if err != nil {
		t.log.Error("Redis getdel failed", slog.String("err", err.Error()))
		return nil, status.Error(codes.Internal, "storage error")
	}

	var identity proto.NormalizedIdentity
	if err := json.Unmarshal([]byte(data), &identity); err != nil {
		t.log.Error("Corrupt identity data", slog.String("err", err.Error()))
		return nil, status.Error(codes.Internal, "corrupt data")
	}

	t.log.Info("token exchanged", slog.String("user_id", identity.UserId))
	return &proto.ExchangeTokenResponse{Valid: true, Identity: &identity}, nil
}
