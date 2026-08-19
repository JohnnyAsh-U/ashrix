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

	registry    *registry.GatewayRegistry
	redisClient *redis.Client
	distributor *policy.PolicyDistributor
	policyStore policy.Repository
	gatewayRepo gateway.Repository
	PkiCARepo pkica.Repository
	connectorRepo connector.Repository
	// dbQueries   *store.Queries
	log         *slog.Logger
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
				ID:        uuid.Must(uuid.Parse(p.Hello.GatewayId)),
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
				GatewayID:        uuid.Must(uuid.Parse(p.CmdAck.GatewayId)),
				LastAckedSeq:  func () int64  {
					fmt.Println(p.CmdAck)
					seq,err:= strconv.ParseInt(p.CmdAck.CmdId,10,64)
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
