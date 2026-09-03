package grpc

import (
	"context"
	"log/slog"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

func UnaryTLSInterceptor(log *slog.Logger) grpc.UnaryServerInterceptor {

	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {

		tlsInfo, err := TLSInfoFromContext(ctx)

		if err != nil {

			log.Warn(
				"gRPC request without TLS authentication",
				"method",
				info.FullMethod,
				"error",
				err,
			)

			return nil, status.Error(
				codes.Unauthenticated,
				"TLS authentication required",
			)
		}

		// At this point TLS has been verified.
		//
		// You can now extract the client certificate and
		// authenticate the Ashrix identity.

		_ = tlsInfo

		return handler(ctx, req)
	}
}

func StreamTLSInterceptor(
	log *slog.Logger,
) grpc.StreamServerInterceptor {

	return func(
		srv interface{},
		stream grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {

		ctx := stream.Context()

		tlsInfo, err := TLSInfoFromContext(ctx)

		if err != nil {

			log.Warn(
				"gRPC stream without TLS authentication",
				"method",
				info.FullMethod,
				"error",
				err,
			)

			return status.Error(
				codes.Unauthenticated,
				"TLS authentication required",
			)
		}

		_ = tlsInfo

		return handler(
			srv,
			stream,
		)
	}
}

// TLSInfoFromContext extracts the TLS authentication information
// injected by InjectTLSInfo.
func TLSInfoFromContext(
	ctx context.Context,
) (credentials.TLSInfo, error) {

	p, ok := peer.FromContext(ctx)

	if !ok || p == nil {
		return credentials.TLSInfo{}, status.Error(
			codes.Unauthenticated,
			"no peer information",
		)
	}

	tlsInfo, ok := p.AuthInfo.(credentials.TLSInfo)

	if !ok {
		return credentials.TLSInfo{}, status.Error(
			codes.Unauthenticated,
			"no TLS Info",
		)
	}

	return tlsInfo, nil
}