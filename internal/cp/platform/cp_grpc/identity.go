package cp_grpc

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

type ctxKey string

const GatewayIDKey ctxKey = "gateway_id"

type wrappedStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *wrappedStream) Context() context.Context {
	return w.ctx
}

func StreamIdentityInterceptor(
	srv any,
	ss grpc.ServerStream,
	info *grpc.StreamServerInfo,
	handler grpc.StreamHandler,
) error {
	ctx := ss.Context()
	peerInfo, ok := peer.FromContext(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "no peer info")
	}
	tlsInfo, ok := peerInfo.AuthInfo.(credentials.TLSInfo)

	if !ok {
		return status.Error(codes.Unauthenticated, "no TLS Info")
	}

	cert := tlsInfo.State.PeerCertificates[0]
	org := cert.Subject.Organization[0]
	ou := cert.Subject.OrganizationalUnit[0]
	cn := cert.Subject.CommonName

	newCtx := context.WithValue(ctx, GatewayIDKey, cn)

	if org != "Ashrix" || ou != "Gateway" || cn == "" {
		return status.Error(codes.Unauthenticated, "Identity check failed")
	}
	fmt.Println(cert.Subject)

	return handler(srv, &wrappedStream{
		ServerStream: ss,
		ctx:          newCtx,
	})

}
