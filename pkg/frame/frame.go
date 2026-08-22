package frame

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net/http"

	pb "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	"google.golang.org/protobuf/proto"
)

const (
	ProtocolVersion uint8 = 0
	MaxFrameSize          = 16 << 20 // 16 MiB control frame
)

type FrameType uint8

const (
	FrameHTTPRequest FrameType = iota + 1
	FrameHTTPResponse
	FrameWSOpen
	FrameWSOpenResponse
	FrameWSData
	FrameWSPing
	FrameWSPong
	FrameWSClose
	FrameError
)


func WriteFrame(w io.Writer, typ pb.FrameType, payload proto.Message) error {

	data, err := proto.Marshal(payload)

	if err != nil {
		return fmt.Errorf("marshal frame: %w", err)
	}

	if len(data) > MaxFrameSize {
		return errors.New("frame too large")
	}

	header := make([]byte, 6)
	header[0] = ProtocolVersion
	header[1] = byte(typ)
	binary.BigEndian.PutUint32(header[2:], uint32(len(data)))

	if _, err := w.Write(header); err != nil {
		return fmt.Errorf("write frame header: %w", err)
	}

	if _, err := w.Write(data); err != nil {
		return fmt.Errorf("write frame payload: %w", err)
	}

	return nil

}

func ReadFrame(r io.Reader) (FrameType, []byte, error) {

	var header [6]byte

	if _, err := io.ReadFull(r, header[:]); err != nil {
		return 0, nil, err
	}

	if header[0] != ProtocolVersion {
		return 0, nil, fmt.Errorf("unsupported protocol version %d", header[0])

	}

	typ := FrameType(header[1])

	length := binary.BigEndian.Uint32(header[2:])
	if length > MaxFrameSize {
		return 0, nil, errors.New("frame too large")
	}

	payload := make([]byte, length)
	if _, err := io.ReadFull(r, payload); err != nil {
		return 0, nil, err
	}

	return typ, payload, nil
}

func DecodeFrame(payload []byte, dst proto.Message) error {

	if err := proto.Unmarshal(payload, dst); err != nil {
		return fmt.Errorf("decode frame: %w", err)
	}

	return nil
}


// headersToProto converts standard Go HTTP headers into the Protobuf map type
func HeadersToProto(h http.Header) map[string]*pb.HeaderList {
	if h == nil {
		return nil
	}
	out := make(map[string]*pb.HeaderList, len(h))
	for k, values := range h {
		// Copy the slice so the original request doesn't share memory with the proto message
		copied := make([]string, len(values))
		copy(copied, values)

		out[k] = &pb.HeaderList{
			Values: copied,
		}
	}
	return out
}

