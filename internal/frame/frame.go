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
	MaxFrameSize = 16 << 20 // 16 MiB control frame
)

var FrameTooLarge = errors.New("frame too large")

func WriteFrame(w io.Writer, payload proto.Message) error {

	data, err := proto.Marshal(payload)

	if err != nil {
		return fmt.Errorf("marshal frame: %w", err)
	}

	if len(data) > MaxFrameSize {
		return FrameTooLarge
	}

	header := make([]byte, 4)
	binary.BigEndian.PutUint32(header[0:], uint32(len(data)))

	if err := writeFull(w, header); err != nil {
		return fmt.Errorf("write frame header: %w", err)
	}

	if err := writeFull(w, data); err != nil {
		return fmt.Errorf("write frame payload: %w", err)
	}

	return nil

}

// writeFull handles legal short writes from io.Writer implementations.
func writeFull(w io.Writer, p []byte) error {
	for len(p) > 0 {
		n, err := w.Write(p)
		if n > 0 {
			p = p[n:]
		}
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
	}
	return nil
}

func ReadFrame(r io.Reader) ([]byte, error) {

	var header [4]byte

	if _, err := io.ReadFull(r, header[:]); err != nil {
		return nil, err
	}

	length := binary.BigEndian.Uint32(header[0:])
	if length > MaxFrameSize {
		return nil, FrameTooLarge
	}

	payload := make([]byte, length)
	if _, err := io.ReadFull(r, payload); err != nil {
		return nil, err
	}

	return payload, nil
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
