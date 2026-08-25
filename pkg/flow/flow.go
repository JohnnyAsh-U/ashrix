package flow

import (
	"context"
	"errors"
	"io"
	"sync"
)

type FlowType string

const (
	FlowUserToApp FlowType = "USER_TO_APP"
	FlowAppToApp  FlowType = "APP_TO_APP"
)

type Protocol string

const (
	ProtocolTCP Protocol = "TCP"
	ProtocolUDP Protocol = "UDP"
)

type EndpointType string

const (
	EndpointUser EndpointType = "USER"
	EndpointApp  EndpointType = "APP"
)

type Endpoint struct {
	Type        EndpointType
	PrincipalID string
	DeviceID    string
	AppID       string
}

type OpenRequest struct {
	Version     int
	FlowID      string
	FlowType    FlowType
	Protocol    Protocol
	Source      Endpoint
	Destination Endpoint

	HTTPMethod  string
	HTTPPath    string
	HTTPHost    string
	HTTPQuery   string
	HTTPHeaders map[string][]string
	BodyLength  int64
	StreamType  int32 // Maps to proto.RequestType

	SocksUsername string
	SocksPassword string
}

type Stream interface {
	io.Reader
	io.Writer
	Close() error
	CloseRead() error
	CloseWrite() error
	Context() context.Context
}

type RelayResult struct {
	BytesAToB int64
	BytesBToA int64
	Err       error
}

// Relay handles bidirectional piping between two flow.Streams.
// It supports proper context cancellation, EOF, half-close semantics,
// and returns exact byte counts and transfer errors.
func Relay(ctx context.Context, a Stream, b Stream) RelayResult {
	var result RelayResult
	var wg sync.WaitGroup
	wg.Add(2)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Monitor context cancellation to force-close connections
	go func() {
		<-ctx.Done()
		_ = a.Close()
		_ = b.Close()
	}()

	var errA, errB error
	var once sync.Once
	setErr := func(err error) {
		if err != nil && !errors.Is(err, io.EOF) {
			once.Do(func() {
				result.Err = err
				cancel() // cancel the other direction
			})
		}
	}

	// Copy a -> b
	go func() {
		defer wg.Done()
		n, err := io.Copy(b, a)
		result.BytesAToB = n
		if err != nil {
			errA = err
			setErr(err)
		}
		_ = b.CloseWrite()
	}()

	// Copy b -> a
	go func() {
		defer wg.Done()
		n, err := io.Copy(a, b)
		result.BytesBToA = n
		if err != nil {
			errB = err
			setErr(err)
		}
		_ = a.CloseWrite()
	}()

	wg.Wait()

	if result.Err == nil {
		if errA != nil && !errors.Is(errA, io.EOF) {
			result.Err = errA
		} else if errB != nil && !errors.Is(errB, io.EOF) {
			result.Err = errB
		}
	}

	return result
}
