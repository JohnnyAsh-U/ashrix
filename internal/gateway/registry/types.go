package registry

import (
	gen "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
)



//Management stream is the interface to push commands to a connected connector.
//satisfied automatically by the GatewayConnectorEnvelope from the proto

type ManagementStream interface {
	Send(*gen.GatewayConnectorEnvelope) error
}



//Tunnel Session is the minimal interface for the dataplane connection to a connector
//(Quic or gRPC tunnel). Kept separate from ManagementStream deliberately
//they are different streams, different lifecycles.

type TunnelSession interface {
	OpenStream() (Stream, error)
	Close() error
}

type Stream interface {
	Read(p []byte) (int, error)
	Write(p []byte) (int, error)
	Close() error
}
