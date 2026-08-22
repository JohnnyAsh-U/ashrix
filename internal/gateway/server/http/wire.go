package http_proxy

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/registry"
	proto "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
)

const maxEvelopeSize = 64*1024 //64KB envelopes are metadata, never bodies

func writeEvelope(stream registry.Stream, envelope interface{}) error {
	data, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("Marshall Envelope: %w", err)
	}

	if len(data) > maxEvelopeSize {
		// Envelopes are metadata - headers, method, path
		// if this fires means something is wrong(request with thousands of headers)
		// fails loudly instead of silently truncating, which would corrupt the protocol
		return fmt.Errorf("Envelope too large: %d bytes (max: %d)", len(data), maxEvelopeSize)
	}

	//Length prefix is 4 bytes, big endian
	lengthPrefix := make([]byte, 4)
	binary.BigEndian.PutUint32(lengthPrefix, uint32(len(data)))

	//Write length prefix
	if _, err := stream.Write(lengthPrefix); err != nil {
		return fmt.Errorf("Write Envelope Length Prefix: %w", err)
	}

	//Write envelope data
	if _, err := stream.Write(data); err != nil {
		return fmt.Errorf("Write Envelope Data: %w", err)
	}

	return nil
}

//Read envelopes reads a length-prefixed JSON envelope from the stream and unmarshals it into the provided interface.
func readEnvelope(stream registry.Stream, envelope interface{}) error {
	//Read length prefix
	lengthPrefix := make([]byte, 4)
	if _, err := io.ReadFull(stream, lengthPrefix); err != nil {
		return fmt.Errorf("Read Envelope Length Prefix: %w", err)
	}

	length := binary.BigEndian.Uint32(lengthPrefix)
	if length > maxEvelopeSize {
		return fmt.Errorf("Envelope too large: %d bytes (max: %d)", length, maxEvelopeSize)
	}

	data := make([]byte, length)
	if _, err := io.ReadFull(stream, data); err != nil {
		return fmt.Errorf("Read Envelope Data: %w", err)
	}

	if err := json.Unmarshal(data, envelope); err != nil {
		return fmt.Errorf("Unmarshal Envelope: %w", err)
	}

	return nil
}

func flattenHeaders(h http.Header) map[string]string {
	out := make(map[string]string, len(h))
	for k, vals := range h {
		if len(vals) > 0 {
			out[k] = vals[0]
		}
	}
	return out
}

func cloneHeaders(h http.Header) map[string]*proto.HeaderList {
	out := make(map[string]*proto.HeaderList, len(h))

	for k, vals := range h {
		copied := make([]string, len(vals))
		copy(copied, vals)
		out[k] = &proto.HeaderList{
			Values: copied,
		}
	}

	return out
}


func isWebSocketRequest(r *http.Request) bool {
	fmt.Println("==============================================")
	fmt.Println(r.Header.Get("Connection"))
	fmt.Println(r.Header.Get("Upgrade"))
	fmt.Println("==============================================")
	return strings.EqualFold(r.Header.Get("Upgrade"), "websocket")
}