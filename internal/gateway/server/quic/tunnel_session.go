package quic

import (
	"context"
	"fmt"

	"github.com/JohnnyAsh-U/ashrix-api/pkg/flow"
	"github.com/JohnnyAsh-U/ashrix-api/pkg/frame"
	proto "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	"github.com/quic-go/quic-go"
)

type quicTunnelSession struct {
	conn *quic.Conn
}

func (s *quicTunnelSession) OpenStream(ctx context.Context, req flow.OpenRequest) (flow.Stream, error) {
	stream, err := s.conn.OpenStreamSync(ctx)
	if err != nil {
		return nil, fmt.Errorf("Open Tunnel Stream: %w", err)
	}

	var streamType proto.RequestType
	if req.StreamType != 0 {
		streamType = proto.RequestType(req.StreamType)
	} else if req.FlowType == flow.FlowUserToApp {
		streamType = proto.RequestType_HTTP_REQUEST
	}

	var flowType proto.FlowType
	if req.FlowType == flow.FlowAppToApp {
		flowType = proto.FlowType_APP_TO_APP
	} else {
		flowType = proto.FlowType_USER_TO_APP
	}

	envelope := proto.StreamFrame{
		RequestId:   req.FlowID,
		AppId:       req.Destination.AppID,
		StreamType:  streamType,
		FlowType:    flowType,
		Method:      req.HTTPMethod,
		Host:        req.HTTPHost,
		Path:        req.HTTPPath,
		Query:       req.HTTPQuery,
		Headers:     frame.HeadersToProto(req.HTTPHeaders),
		BodyLength:  req.BodyLength,
	}

	if req.FlowType == flow.FlowUserToApp {
		envelope.UserId = req.Source.PrincipalID
		envelope.UserEmail = req.Source.PrincipalID
	} else {
		envelope.ConnectorId = req.Source.PrincipalID
	}

	adapter := &quicStreamAdapter{
		stream: stream,
		ctx:    ctx,
	}

	if err := frame.WriteFrame(adapter, &envelope); err != nil {
		stream.Close()
		return nil, fmt.Errorf("failed to write open request frame: %w", err)
	}

	return adapter, nil
}

func (s *quicTunnelSession) Close() error {
	return s.conn.CloseWithError(0, "Closing")
}

type quicStreamAdapter struct {
	stream *quic.Stream
	ctx    context.Context
}

func (s *quicStreamAdapter) Read(p []byte) (int, error)  { return s.stream.Read(p) }
func (s *quicStreamAdapter) Write(p []byte) (int, error) { return s.stream.Write(p) }
func (s *quicStreamAdapter) Close() error                { return s.stream.Close() }
func (s *quicStreamAdapter) CloseRead() error {
	s.stream.CancelRead(0)
	return nil
}
func (s *quicStreamAdapter) CloseWrite() error {
	s.stream.CancelWrite(0)
	return nil
}
func (s *quicStreamAdapter) Context() context.Context {
	if s.ctx != nil {
		return s.ctx
	}
	return context.Background()
}
