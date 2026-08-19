package cp_grpc

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"strconv"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/connector"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/database/store"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/gateway"
	pkica "github.com/JohnnyAsh-U/ashrix-api/internal/cp/pki_ca"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/cp_grpc/dispatcher"
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
	PkiCARepo     pkica.Repository
	connectorRepo connector.Repository
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

	// Wait for the initial Hello message from the gateway
	msg, err := stream.Recv()
	if err != nil {
		return fmt.Errorf("recv hello: %w", err)
	}

	hello := msg.GetHello()
	if hello == nil {
		return fmt.Errorf("expected hello message, got %T", msg.Payload)
	}

	gatewayID := hello.GatewayId
	tenantID := hello.TenantId
	currentPolicyVersion := hello.PolicyVersion

	ctx, cancel := context.WithCancel(stream.Context())
	defer cancel()

	conn := &registry.GatewayConn{
		GatewayID:            gatewayID,
		TenantID:             tenantID,
		Stream:               stream,
		ConnectedAt:          time.Now(),
		LastSeen:             time.Now(),
		CurrentPolicyVersion: uint64(currentPolicyVersion),
		Ctx:                  ctx,
		Cancel:               cancel,
	}

	// Register the connection
	s.registry.Register(conn)
	defer s.registry.Unregister(gatewayID)

	log.Printf("gateway connected: %s (tenant=%s, policy_version=%d)",
		gatewayID, tenantID, currentPolicyVersion)

	s.registry.HandleHello(gatewayID)

	//Get the last ack for gateway
	lastAckSeq, err := s.gatewayRepo.GetLastAckedSeqForGateway(ctx, gatewayUUID)

	fmt.Println(lastAckSeq)

	if err != nil {
		s.log.Error("Last Ack Seq Error:", slog.String("err", err.Error()))
	}

	events, err := s.gatewayRepo.GetLatestGatewayEventsByCommand(ctx, store.GetLatestGatewayEventsByCommandParams{
		GatewayID: gatewayUUID,
		Seq: lastAckSeq,
	})

	if err != nil {
		s.log.Error(
			"failed to get latest gateway events",
			slog.String("gateway_id", gatewayUUID.String()),
			slog.String("err", err.Error()),
		)

		return status.Error(codes.Internal, "failed to restore gateway state")
	}

	for _, event := range events {
		cmd, err := dispatcher.GatewayEventToCmd(event)
		fmt.Println(event.Command, event.Seq)
		if err != nil {
			s.log.Error(
				"failed to convert gateway event",
				slog.Int64("seq", event.Seq),
				slog.String("command", event.Command),
				slog.String("err", err.Error()),
			)

			return status.Error(codes.Internal, "failed to restore gateway state")
		}

		envelope := &proto.CPEnvelope{
			SentAt: timestamppb.Now(),
			Payload: &proto.CPEnvelope_Cmd{
				Cmd: cmd,
			},
		}

		if err := stream.Send(envelope); err != nil {
			return fmt.Errorf(
				"failed to send gateway event seq=%d: %w",
				event.Seq,
				err,
			)
		}
	}

	//For revoke session we replay all the commands from the last acked seq since it is incremental

	//Get the latest gateway events by command and send
	// events, err := s.gatewayRepo.GetLatestGatewayEventsByCommand(ctx, gatewayUUID)
	// if err == nil {
	// 	for _, event := range events {
	// 		if event.Command == string(dispatcher.CmdConnectorSync) {
	// 			// err = stream.Send(&proto.GatewayConnectorEnvelope{
	// 			// 	Payload: &proto.GatewayEnvelope_Cmd{
	// 			// 		Cmd: &proto.Command{
	// 			// 			Payload: &proto.Command_RotateGatewayCert{
	// 			// 				RotateGatewayCert: &proto.RotateGatewayCertCmd{},
	// 			// 			},
	// 			// 		},
	// 			// 	},
	// 			// })
	// 			err = stream.Send(&proto.CPEnvelope{
	// 				SentAt: timestamppb.Now(),
	// 				Payload: &proto.CPEnvelope_Cmd{
	// 					Cmd: &proto.Command{
	// 						CmdId: string(rune(event.Seq)),
	// 						Payload: &proto.ConnectorSyncCmd{
	// 						},
	// 					},
	// 				},
	// 			})
	// 		}
	// 	}
	// }

	// // Push CRL entries list immediately
	// crls, err := s.PkiCARepo.GetCRLEntry(ctx)
	// if err == nil {
	// 	var serials []string
	// 	for _, c := range crls {
	// 		serials = append(serials, c.SerialNumber)
	// 	}
	// 	crlSyncEnvelope := &proto.CPEnvelope{
	// 		SentAt: timestamppb.Now(),
	// 		Payload: &proto.CPEnvelope_Cmd{
	// 			Cmd: &proto.Command{
	// 				Payload: &proto.Command_CrlSync{
	// 					CrlSync: &proto.CrlSyncCmd{
	// 						RevokedSerialNumbers: serials,
	// 					},
	// 				},
	// 			},
	// 		},
	// 	}
	// 	if err := stream.Send(crlSyncEnvelope); err != nil {
	// 		s.log.Error("failed to send initial CRL sync", slog.String("err", err.Error()))
	// 	}
	// }

	// // Push authorized connectors list immediately
	// conns, err := s.connectorRepo.ListActiveConnectorsByGateway(ctx, gatewayUUID)
	// if err == nil {
	// 	var connectorInfos []*proto.ConnectorInfo
	// 	for _, c := range conns {
	// 		connectorInfos = append(connectorInfos, &proto.ConnectorInfo{
	// 			Id:     c.ID.String(),
	// 			Status: c.Status,
	// 		})
	// 	}
	// 	connSyncEnvelope := &proto.CPEnvelope{
	// 		SentAt: timestamppb.Now(),
	// 		Payload: &proto.CPEnvelope_Cmd{
	// 			Cmd: &proto.Command{
	// 				Payload: &proto.Command_ConnectorSync{
	// 					ConnectorSync: &proto.ConnectorSyncCmd{
	// 						Connectors: connectorInfos,
	// 					},
	// 				},
	// 			},
	// 		},
	// 	}
	// 	if err := stream.Send(connSyncEnvelope); err != nil {
	// 		s.log.Error("failed to send initial connector sync", slog.String("err", err.Error()))
	// 	}
	// }

	tenantUUID, err := uuid.Parse(tenantID)
	if err != nil {
		log.Printf("Invalid tenant ID: %s", tenantID)
		return fmt.Errorf("invalid tenant ID: %w", err)
	}

	// If the gateway is behind on policy, push the latest immediately
	if currentPolicyVersion < s.distributor.LatestVersion(tenantUUID) {
		go s.distributor.PushToGateway(ctx, conn, tenantUUID)
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
		fmt.Println(err)
		switch p := msg.Payload.(type) {
		case *proto.GatewayEnvelope_Hello:
			log.Println("Gateway Connected", p.Hello.GatewayId)
			//Update the binary version on the gateway repo
			_, err := s.gatewayRepo.UpdateGatewayBinaryVersion(ctx, store.UpdateGatewayBinaryVersionParams{
				ID:      uuid.Must(uuid.Parse(p.Hello.GatewayId)),
				Version: pgtype.Text{Valid: true, String: p.Hello.BinaryVersion},
			})
			if err != nil {
				log.Printf("Failed to update gateway binary version %v", err)
			} else {
				log.Printf("Gateway binary version updated successfully %v", p.Hello.GatewayId)
			}
			ack := &proto.CPEnvelope{
				Payload: &proto.CPEnvelope_HelloAck{
					HelloAck: &proto.HelloAck{
						ServerVersion: "1.0.0",
						ServerTime:    timestamppb.Now(),
					},
				},
			}
			if err := stream.Send(ack); err != nil {
				log.Printf("Failed to send Hello Ack %v", err)
			}

		case *proto.GatewayEnvelope_Heartbeat:
			log.Println("HeartBeat", p.Heartbeat.Seq)
			// Update the db last seen
			_, err := s.gatewayRepo.UpdateGatewayHeartBeat(ctx, uuid.Must(uuid.Parse(p.Heartbeat.GatewayId)))
			if err != nil {
				log.Printf("Failed to update gateway last seen %v", err)
			} else {
				log.Printf("Gateway last seen updated successfully %v", p.Heartbeat.GatewayId)
			}

		case *proto.GatewayEnvelope_CmdAck:
			log.Println("Command Ack", p.CmdAck)
			_, err := s.gatewayRepo.CreateGatewayEventAck(ctx, store.CreateGatewayEventAcksParams{
				GatewayID: uuid.Must(uuid.Parse(p.CmdAck.GatewayId)),
				LastAckedSeq: func() int64 {
					fmt.Println(p.CmdAck)
					seq, err := strconv.ParseInt(p.CmdAck.CmdId, 10, 64)
					if err != nil {
						log.Println("Failed to parse command ID", slog.String("err", err.Error()))
						return 0
					} else {
						return seq
					}
				}(),
			})
			if err != nil {
				log.Printf("Failed to create gateway event ack %v", err)
			} else {
				log.Printf("Gateway event ack created successfully %v", p.CmdAck.GatewayId)
			}

		default:
			log.Printf("Unknown message")
		}
	}
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

	lastAck, err :=
		s.gatewayRepo.GetLastAckedSeqForGateway(
			ctx,
			gatewayID,
		)

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

	events, err :=
		s.gatewayRepo.ListGatewayEventsAfter(
			ctx,
			store.ListGatewayEventsAfterParams{
				GatewayID: gatewayID,
				Seq:       lastAck,
			},
		)

	if err != nil {
		return fmt.Errorf(
			"load gateway events: %w",
			err,
		)
	}

	if len(events) == 0 {
		return nil
	}

	// --------------------------------------------
	// 3. Compact complete snapshots
	// --------------------------------------------

	events =
		dispatcher.CompactGatewayEvents(
			events,
		)

	// --------------------------------------------
	// 4. Send in original sequence order
	// --------------------------------------------

	for _, event := range events {

		cmd, err :=
			dispatcher.GatewayEventToCmd(
				event,
			)

		if err != nil {
			return fmt.Errorf(
				"convert event %d: %w",
				event.Seq,
				err,
			)
		}

		envelope := &proto.CPEnvelope{
			Seq: event.Seq,

			EventId: event.EventID.String(),

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


func (s *cpServer) receiveLoop(
	ctx context.Context,
	stream proto.ControlPlaneService_ConnectServer,
	conn *registry.GatewayConn,
) error {

	for {

		msg, err := stream.Recv()

		if err == io.EOF {
			s.log.Info(
				"gateway disconnected",
				slog.String(
					"gateway_id",
					conn.GatewayID,
				),
			)

			return nil
		}

		if err != nil {
			return fmt.Errorf(
				"gateway stream receive: %w",
				err,
			)
		}

		conn.LastSeen = time.Now()

		switch p := msg.Payload.(type) {

		case *proto.GatewayEnvelope_Hello:

			if err := s.handleHello(
				ctx,
				conn,
				p.Hello,
			); err != nil {
				return err
			}

		case *proto.GatewayEnvelope_Heartbeat:

			if err := s.handleHeartbeat(
				ctx,
				conn,
				p.Heartbeat,
			); err != nil {
				s.log.Warn(
					"heartbeat handling failed",
					"error", err,
				)
			}

		case *proto.GatewayEnvelope_CmdAck:

			if err := s.handleCommandAck(
				ctx,
				conn,
				p.CmdAck,
			); err != nil {

				s.log.Error(
					"command ACK handling failed",
					"error", err,
				)

				return err
			}

		default:

			s.log.Warn(
				"unknown gateway message",
				"gateway_id",
				conn.GatewayID,
			)
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
		s.gatewayRepo.GetLatestGatewayEventSeq(
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
		s.gatewayRepo.AckGatewayEvents(
			ctx,
			store.AckGatewayEventsParams{
				GatewayID:     gatewayID,
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

func (s *cpServer) handleHello(
	ctx context.Context,
	conn *registry.GatewayConn,
	hello *proto.Hello,
) error {

	if hello.GatewayId != conn.GatewayID {
		return status.Error(
			codes.Unauthenticated,
			"gateway identity mismatch",
		)
	}

	gatewayID, err :=
		uuid.Parse(conn.GatewayID)
if err != nil {
		return err
	}

	_, err =
		s.gatewayRepo.UpdateGatewayBinaryVersion(
			ctx,
			store.UpdateGatewayBinaryVersionParams{
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

	return conn.Send(
		&proto.CPEnvelope{
			Payload: &proto.CPEnvelope_HelloAck{
				HelloAck: &proto.HelloAck{
					ServerVersion: "1.0.0",
					ServerTime:    timestamppb.Now(),
				},
			},
		},
	)
}


func (s *cpServer) handleHeartbeat(
	ctx context.Context,
	conn *registry.GatewayConn,
	heartbeat *proto.Heartbeat,
) error {

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

	return err
}


func (s *cpServer) Connect(
	stream proto.ControlPlaneService_ConnectServer,
) error {

	// ==================================================
	// 1. Authenticate mTLS identity
	// ==================================================

	gw, err :=
		s.authenticateGateway(stream)

	if err != nil {
		return err
	}

	// ==================================================
	// 2. Receive Hello
	// ==================================================

	hello, err :=
		s.receiveHello(stream)

	if err != nil {
		return err
	}

	// ==================================================
	// 3. Verify Hello identity against certificate
	// ==================================================

	gatewayID := gw.ID.String()

	if hello.GatewayId != gatewayID {

		return status.Error(
			codes.Unauthenticated,
			"gateway identity mismatch",
		)
	}

	// ==================================================
	// 4. Register connection
	// ==================================================

	ctx, cancel :=
		context.WithCancel(
			stream.Context(),
		)

	defer cancel()

	conn :=
		registry.NewGatewayConn(
			ctx,
			cancel,
			stream,
			gatewayID,
			hello.TenantId,
			uint64(hello.PolicyVersion),
		)

	if _, exists :=
		s.registry.GetConnection(gatewayID); exists {

		return status.Error(
			codes.AlreadyExists,
			"gateway already connected",
		)
	}

	s.registry.Register(conn)

	defer s.registry.Unregister(
		gatewayID,
	)

	s.log.Info(
		"gateway connected",
		slog.String(
			"gateway_id",
			gatewayID,
		),
		slog.String(
			"tenant_id",
			hello.TenantId,
		),
		slog.Uint64(
			"policy_version",
			uint64(hello.PolicyVersion),
		),
	)

	// ==================================================
	// 5. Reconcile durable gateway events
	// ==================================================

	if err := s.reconcileGateway(
		ctx,
		conn,
	); err != nil {

		s.log.Error(
			"gateway reconciliation failed",
			slog.String(
				"gateway_id",
				gatewayID,
			),
			slog.String(
				"error",
				err.Error(),
			),
		)

		return status.Error(
			codes.Internal,
			"gateway reconciliation failed",
		)
	}

	// ==================================================
	// 6. Policy reconciliation
	// ==================================================

	tenantUUID, err :=
		uuid.Parse(hello.TenantId)

	if err != nil {
		return status.Error(
			codes.InvalidArgument,
			"invalid tenant ID",
		)
	}

	if uint64(hello.PolicyVersion) <
		s.distributor.LatestVersion(
			tenantUUID,
		) {

		go func() {

			if err :=
				s.distributor.PushToGateway(
					ctx,
					conn,
					tenantUUID,
				); err != nil {

				s.log.Error(
					"policy reconciliation failed",
					slog.String(
						"gateway_id",
						gatewayID,
					),
					slog.String(
						"error",
						err.Error(),
					),
				)
			}

		}()
	}

	// ==================================================
	// 7. Normal stream
	// ==================================================

	return s.receiveLoop(
		ctx,
		stream,
		conn,
	)
}

func (t *cpServer) ExchangeToken(ctx context.Context, req *proto.ExchangeTokenRequest) (*proto.ExchangeTokenResponse, error) {
	if req.TokenHash == "" || req.GatewayName == "" {
		return nil, status.Error(codes.InvalidArgument, "token_hash required")
	}
	key := fmt.Sprintf("session:%s:%s", req.GatewayName, req.TokenHash)

	fmt.Println(key)
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
