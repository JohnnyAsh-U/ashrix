package cp_grpc

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	// "time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/cp_grpc/registry"
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

	// gatewayID := hello.GatewayId
	// gatewayType := GatewayType(hello.GatewayType) // "hosted" or "self_hosted"
	// currentPolicyVersion := hello.CurrentPolicyVersion

	// ctx, cancel := context.WithCancel(stream.Context())
	// defer cancel()


	// conn := &GatewayConn{
	// 	GatewayID:            gatewayID,
	// 	GatewayType:          gatewayType,
	// 	Stream:               stream,
	// 	ConnectedAt:          time.Now(),
	// 	LastSeen:             time.Now(),
	// 	CurrentPolicyVersion: currentPolicyVersion,
	// 	ctx:                  ctx,
	// 	cancel:               cancel,
	// }

		// Register the connection
	// s.registry.Register(conn)
	// defer s.registry.Unregister(gatewayID)

	// log.Printf("gateway connected: %s (type=%s, tenant=%s, policy_version=%d)",
	// 	gatewayID, gatewayType, tenantID, currentPolicyVersion)

	// // For hosted gateways, the Hello includes the list of tenants it serves
	// if gatewayType == GatewayTypeHosted && len(hello.ServedTenants) > 0 {
	// 	s.registry.HandleHello(gatewayID, hello.ServedTenants)
	// }

	// // If the gateway is behind on policy, push the latest immediately
	// if currentPolicyVersion < s.distributor.LatestVersion(tenantID) {
	// 	go s.distributor.PushToGateway(ctx, conn, tenantID)
	// } 



	fmt.Println("Said Hello")
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
