package frame

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"testing"

	"google.golang.org/protobuf/types/known/wrapperspb"
)

type shortWriter struct {
	bytes.Buffer
	max int
}

func (w *shortWriter) Write(p []byte) (int, error) {
	if len(p) > w.max {
		p = p[:w.max]
	}
	return w.Buffer.Write(p)
}

type zeroWriter struct{}

func (zeroWriter) Write([]byte) (int, error) { return 0, nil }

type failingWriter struct{ err error }

func (w failingWriter) Write([]byte) (int, error) { return 0, w.err }

func TestWriteReadFrameRoundTrip(t *testing.T) {
	want := wrapperspb.String("hello Ashrix")
	var buf bytes.Buffer
	if err := WriteFrame(&buf, want); err != nil {
		t.Fatalf("WriteFrame() error = %v", err)
	}
	payload, err := ReadFrame(&buf)
	if err != nil {
		t.Fatalf("ReadFrame() error = %v", err)
	}
	got := new(wrapperspb.StringValue)
	if err := DecodeFrame(payload, got); err != nil {
		t.Fatalf("DecodeFrame() error = %v", err)
	}
	if got.Value != want.Value {
		t.Fatalf("decoded value = %q, want %q", got.Value, want.Value)
	}
}

func TestWriteFrameHandlesShortWrites(t *testing.T) {
	w := &shortWriter{max: 2}
	if err := WriteFrame(w, wrapperspb.String("short writes")); err != nil {
		t.Fatalf("WriteFrame() error = %v", err)
	}
	payload, err := ReadFrame(&w.Buffer)
	if err != nil {
		t.Fatalf("ReadFrame() error = %v", err)
	}
	got := new(wrapperspb.StringValue)
	if err := DecodeFrame(payload, got); err != nil {
		t.Fatalf("DecodeFrame() error = %v", err)
	}
	if got.Value != "short writes" {
		t.Fatalf("value = %q", got.Value)
	}
}

func TestWriteFrameReturnsErrorWhenWriterMakesNoProgress(t *testing.T) {
	if err := WriteFrame(zeroWriter{}, wrapperspb.String("x")); !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("WriteFrame() error = %v, want %v", err, io.ErrShortWrite)
	}
}

func TestWriteFramePropagatesWriterError(t *testing.T) {
	want := errors.New("write failed")
	if err := WriteFrame(failingWriter{err: want}, wrapperspb.String("x")); !errors.Is(err, want) {
		t.Fatalf("WriteFrame() error = %v, want wrapped %v", err, want)
	}
}

func TestReadFrameRejectsTruncatedHeader(t *testing.T) {
	_, err := ReadFrame(bytes.NewReader([]byte{0, 0, 0}))
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("ReadFrame() error = %v, want %v", err, io.ErrUnexpectedEOF)
	}
}

func TestReadFrameRejectsTruncatedPayload(t *testing.T) {
	data := []byte{0, 0, 0, 4, 'a', 'b'}
	_, err := ReadFrame(bytes.NewReader(data))
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("ReadFrame() error = %v, want %v", err, io.ErrUnexpectedEOF)
	}
}

func TestReadFrameRejectsOversizedPayload(t *testing.T) {
	var header [4]byte
	binary.BigEndian.PutUint32(header[:], MaxFrameSize+1)
	_, err := ReadFrame(bytes.NewReader(header[:]))
	if err == nil {
		t.Fatal("ReadFrame() expected oversized-frame error")
	}
}

func TestDecodeFrameRejectsMalformedProtobuf(t *testing.T) {
	// Field 1 (string) with a length that exceeds the available bytes.
	err := DecodeFrame([]byte{0x0a, 0x05, 'x'}, new(wrapperspb.StringValue))
	if err == nil {
		t.Fatal("DecodeFrame() expected malformed protobuf error")
	}
}

func TestReadMultipleFramesFromOneReader(t *testing.T) {
	var buf bytes.Buffer
	for _, value := range []string{"first", "second"} {
		if err := WriteFrame(&buf, wrapperspb.String(value)); err != nil {
			t.Fatal(err)
		}
	}
	for _, want := range []string{"first", "second"} {
		payload, err := ReadFrame(&buf)
		if err != nil {
			t.Fatalf("ReadFrame() error = %v", err)
		}
		got := new(wrapperspb.StringValue)
		if err := DecodeFrame(payload, got); err != nil {
			t.Fatal(err)
		}
		if got.Value != want {
			t.Fatalf("value = %q, want %q", got.Value, want)
		}
	}
}
