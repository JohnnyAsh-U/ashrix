package frame

import (
	"bytes"
	"io"
	"net/http"
	"testing"

	pb "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	"google.golang.org/protobuf/proto"
)

func benchmarkMessage() *pb.StreamFrame {
	return &pb.StreamFrame{
		RequestId:   "request-123456",
		ConnectorId: "connector-a",
		AppId:       "app-a",
		Method:      "POST",
		Host:        "internal-app",
		Path:        "/api/v1/documents",
		Query:       "page=1&limit=100",
		Headers: map[string]*pb.HeaderList{
			"Content-Type": {
				Values: []string{"application/json"},
			},
			"Authorization": {
				Values: []string{"Bearer benchmark-token"},
			},
			"Accept": {
				Values: []string{"application/json"},
			},
			"X-Request-ID": {
				Values: []string{"request-123456"},
			},
		},
	}
}

func BenchmarkFrameWrite(b *testing.B) {
	msg := benchmarkMessage()

	b.ReportAllocs()
	b.SetBytes(int64(proto.Size(msg)))

	for b.Loop() {
		if err := WriteFrame(io.Discard, msg); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkFrameWriteBuffer(b *testing.B) {
	msg := benchmarkMessage()
	var buf bytes.Buffer

	b.ReportAllocs()

	for b.Loop() {
		buf.Reset()

		if err := WriteFrame(&buf, msg); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkFrameRead(b *testing.B) {
	msg := benchmarkMessage()

	var wire bytes.Buffer
	if err := WriteFrame(&wire, msg); err != nil {
		b.Fatal(err)
	}

	data := wire.Bytes()
	reader := bytes.NewReader(data)

	b.ReportAllocs()
	b.SetBytes(int64(len(data)))

	for b.Loop() {
		reader.Reset(data)

		if _, err := ReadFrame(reader); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkFrameDecode(b *testing.B) {
	msg := benchmarkMessage()

	payload, err := proto.Marshal(msg)
	if err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.SetBytes(int64(len(payload)))

	for b.Loop() {
		var decoded pb.StreamFrame

		if err := DecodeFrame(payload, &decoded); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkFrameDecodeWithoutHeaders(b *testing.B) {
	msg := benchmarkMessage()
	msg.Headers = nil

	payload, err := proto.Marshal(msg)
	if err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()

	for b.Loop() {
		var decoded pb.StreamFrame

		if err := DecodeFrame(payload, &decoded); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkHeadersToProto(b *testing.B) {
	headers := make(http.Header)

	headers.Set("Content-Type", "application/json")
	headers.Set("Authorization", "Bearer benchmark-token")
	headers.Set("Accept", "application/json")
	headers.Set("X-Request-ID", "request-123456")
	headers.Set("User-Agent", "Ashrix-Benchmark/1.0")

	b.ReportAllocs()

	for b.Loop() {
		_ = HeadersToProto(headers)
	}
}

func BenchmarkHeadersToProtoNoCopy(b *testing.B) {
	headers := make(http.Header)

	headers.Set("Content-Type", "application/json")
	headers.Set("Authorization", "Bearer benchmark-token")
	headers.Set("Accept", "application/json")
	headers.Set("X-Request-ID", "request-123456")
	headers.Set("User-Agent", "Ashrix-Benchmark/1.0")

	b.ReportAllocs()

	for b.Loop() {
		_ = HeadersToProtoNoCopy(headers)
	}
}

func BenchmarkProtoMarshal(b *testing.B) {
	msg := benchmarkMessage()
	size := proto.Size(msg)
	b.ReportAllocs()

	for b.Loop() {
		buf := make([]byte, 0, size)

		if _, err := (proto.MarshalOptions{}).MarshalAppend(buf, msg); err != nil {
			b.Fatal(err)
		}
	}
}
