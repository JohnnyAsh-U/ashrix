package transport

import (
	"context"
	"io"
)

// Session is multiplexed connection to the gateway
// Implemented by both QUIC, gRPC and websocket(yamux)
type Session interface {

	//OpenStream opens a new outbound stream to the gateway
	//Used for registration, control messages.
	OpenStream(ctx context.Context) (Stream, error)

	//Accept Stream blocks until gateway opens a stream to us
	//Used for: incoming request stream
	AcceptStream(ctx context.Context) (Stream, error)

	//close closes the session and all streams
	Close() error

	//Done returns a channel that closes when session is dead.
	Done() <-chan struct{}

	//TransportName return "quic" or "grpc" or "websocket"
	TransportName() string
}

//Stream is a single bidirectional channel within a Session
//Implements io.ReadWriteCloser

type Stream interface {
	io.ReadWriteCloser

	//Close Write signals end of our writes(half-close)
	//Gateway knows we finished sending the request body
	CloseWrite() error
}
