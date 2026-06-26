package gateway_grpc

import (
	"context"
	"fmt"
	"time"

	proto "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	"go.uber.org/zap"
)

func SendHello(
	ctx context.Context,
	stream proto.ControlPlaneService_ConnectClient,
	gatewayID string,
	log *zap.Logger,
) error {
	log.Info("sending hello to CP")

	if err := stream.Send(&proto.GatewayEnvelope{
		GatewayId: "3",
		Payload: &proto.GatewayEnvelope_Hello{
			Hello: &proto.HelloMessage{
				GatewayId:     gatewayID,
				BinaryVersion: "0",
				PolicyVersion: 0,
				CrlVersion:    0,
				TrustVersion:  0,
			},
		},
	}); err != nil {
		fmt.Println(err)
		return fmt.Errorf("failed to send hello: %w", err)
	}

	// Wait for ACK with timeout
	helloCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	respCh := make(chan error, 1)

	fmt.Println("Here")

	go func() {
		msg, err := stream.Recv()
		if err != nil {
			respCh <- fmt.Errorf("no response to hello: %w", err)
			return
		}
		switch p := msg.Payload.(type) {
		case *proto.CPEnvelope_HelloAck:
			log.Info("CP acknowledged hello",
				zap.String("server_version", p.HelloAck.ServerVersion),
			)
			respCh <- nil
		default:
			respCh <- fmt.Errorf("unexpected CP response type to hello")
		}
	}()

	select {
	case err := <-respCh:
		return err
	case <-helloCtx.Done():
		return &FatalError{
			UserMessage: "CP did not respond to hello within 10 seconds.\n" +
				"Check CP health and network connectivity.",
		}
	}

}
