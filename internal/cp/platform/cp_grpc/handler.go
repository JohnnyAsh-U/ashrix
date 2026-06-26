package cp_grpc

import (
	"fmt"
	"io"
	"log"
	"log/slog"

	proto "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type cpServer struct {
	proto.UnimplementedControlPlaneServiceServer
	log *slog.Logger
}

//Connect handles the bidirectional stream from a gateway
func (s *cpServer) Connect(stream proto.ControlPlaneService_ConnectServer) error {
	fmt.Println("Said Hello")
	for {
		msg, err := stream.Recv()
		if err == io.EOF{
			log.Fatal("Gateway Disconnected Cleanly")
			return nil
		}
		if err != nil {
			log.Println(err)
			return nil
		}
		fmt.Println(err)
		switch p:= msg.Payload.(type) {
		case *proto.GatewayEnvelope_Hello:
			log.Println("Gateway Connected", p.Hello.GatewayId)

			ack := &proto.CPEnvelope{
				Payload: &proto.CPEnvelope_HelloAck{
					HelloAck: &proto.HelloAck{
						ServerVersion: "1.0.0",
						ServerTime: timestamppb.Now(),
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
						ServerTime: timestamppb.Now(),
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