package cp_grpc

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"time"

	// "time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/cp_grpc/registry"
	"github.com/google/uuid"
	// "github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/pki"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/policy"
	proto "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type cpServer struct {
	proto.UnimplementedControlPlaneServiceServer

	registry    *registry.GatewayRegistry
	redisClient *redis.Client
	distributor *policy.PolicyDistributor
	policyStore policy.Repository
	log         *slog.Logger
}

// Connect handles the bidirectional stream from a gateway
func (s *cpServer) Connect(stream proto.ControlPlaneService_ConnectServer) error {

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
	fmt.Println(hello.TenantId)
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
