package proxy

import (
	"bufio"
	"context"
	"io"
	"log/slog"
	"maps"
	"net/http"
	"strings"

	"github.com/JohnnyAsh-U/ashrix-api/internal/frame"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/tunnel"
	proto "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	"github.com/google/uuid"
)

func ProxyRequest(w http.ResponseWriter, r *http.Request, quicTunnel *tunnel.QuicTunnel, log *slog.Logger) {
	ctx := r.Context()

	AppID := AppIDFromCtx(ctx)
	ConnectorID := ConnectorIDFromCtx(ctx)
	requestID := uuid.NewString()
	outboundHeaders := identityHeaders(r.Header, AppID, requestID)
	outboundRequest := r.Clone(r.Context())
	outboundRequest.Header = outboundHeaders

	streamCtx, cancel := context.WithCancel(ctx)
	streamID := uuid.NewString()

	defer func() {
		cancel()
		log.Debug("Tunnel stream closed", slog.String("stream_id", streamID), slog.String("app_id", AppID))
	}()

	streamType := proto.RequestType_HTTP_REQUEST

	if isWebSocketRequest(r) {
		streamType = proto.RequestType_WS_REQUEST
	}

	req := &proto.StreamFrame{
		RequestId:  requestID,
		ConnectorId: ConnectorID,
		AppId:      AppID,
		Method:     r.Method,
		Host:       r.Host,
		Path:       r.URL.Path,
		Query:      r.URL.RawQuery,
		Headers:    frame.HeadersToProto(outboundHeaders),
		BodyLength: r.ContentLength,
		StreamType: streamType,
	}

	// 8. Open a stream to the destination connector.
	tunnel, ok := quicTunnel.GetConnectorTunnel(req.ConnectorId)
	if !ok || tunnel == nil {
		http.Error(w, "No Connectors", 400)
		return
	}

	// stream, err := tunnel.OpenStreamSync(streamCtx, req)
	// if err != nil {
	// 	http.Error(w, "Failed to open Stream", 400)
	// 	return
	// }

	stream, err := tunnel.OpenStreamSync(streamCtx)
	if err != nil {
		http.Error(w, "Open Tunnel Stream", 400)
		return
	}

	if err := frame.WriteFrame(stream, req); err != nil {
		stream.Close()
		http.Error(w, "failed to write open request frame", 502)
		return
	}


	defer stream.Close()

	if !isWebSocketRequest(r) {

		if err := outboundRequest.Write(stream); err != nil {
			log.Error("failed to write request to stream", slog.Any("error", err))
			http.Error(w, "", 500)
			return
		}

		br := bufio.NewReader(stream)

		resp, err := http.ReadResponse(br, r)
		if err != nil {
			http.Error(w, "", 500)
			return
		}

		maps.Copy(w.Header(), resp.Header)
		w.WriteHeader(resp.StatusCode)

		io.Copy(w, resp.Body)
		resp.Body.Close()

	} else {

		hj, ok := w.(http.Hijacker)
		if !ok {
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
		clientConn, bufrw, err := hj.Hijack()
		if err != nil {
			http.Error(w,"", http.StatusInternalServerError)
			return
		}
		defer clientConn.Close()

		if err := outboundRequest.Write(stream); err != nil {
			return
		}

		go func() {
			io.Copy(stream, bufrw)
			stream.Close()
		}()

		br := bufio.NewReader(stream)
		for {
			line, err := br.ReadString('\n')
			if err != nil {
				return
			}
			clientConn.Write([]byte(line))
			if line == "\r\n" || line == "\n" {
				break
			}
		}

		io.Copy(clientConn, br)
	}
}


func isWebSocketRequest(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("Upgrade"), "websocket")
}
