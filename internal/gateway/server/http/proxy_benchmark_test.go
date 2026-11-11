package http_proxy

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/registry"
	proto "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
)

const benchmarkBodySize = 1 << 20

type processSample struct {
	goroutines int
	filedescs  int
}

func sampleProcess() processSample {
	return processSample{
		goroutines: runtime.NumGoroutine(),
		filedescs:  openFileDescriptorCount(),
	}
}

func reportProcessDelta(b *testing.B, before processSample) {
	b.Helper()
	after := sampleProcess()
	b.ReportMetric(float64(after.goroutines-before.goroutines), "goroutine-delta")
	if before.filedescs >= 0 && after.filedescs >= 0 {
		b.ReportMetric(float64(after.filedescs-before.filedescs), "fd-delta")
	}
}

func openFileDescriptorCount() int {
	entries, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		return -1
	}
	return len(entries)
}

func BenchmarkDirectTCPHTTP(b *testing.B) {
	_, upstream := newTestApp(b, "direct-benchmark-app")
	client := &http.Client{Timeout: 30 * time.Second}
	defer client.CloseIdleConnections()
	body := bytes.Repeat([]byte("u"), benchmarkBodySize)

	for _, workload := range []struct {
		name string
		path string
		body []byte
	}{
		{name: "upload-1MiB", path: "/upload", body: body},
		{name: "download-1MiB", path: "/download"},
	} {
		b.Run(workload.name, func(b *testing.B) {
			before := sampleProcess()
			b.ReportAllocs()
			b.SetBytes(benchmarkBodySize)
			perform := func() error {
				return benchmarkHTTPRoundTrip(client, upstream.URL+workload.path, workload.body)
			}
			if err := perform(); err != nil {
				b.Fatal(err)
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := perform(); err != nil {
					b.Fatal(err)
				}
			}
			b.StopTimer()
			reportProcessDelta(b, before)
		})
	}
}

func BenchmarkAshrixHTTPForwarding(b *testing.B) {
	app, _ := newTestApp(b, "ashrix-benchmark-app")
	h := newProxyHarness(b, []*proto.ConnectorApps{app})
	body := bytes.Repeat([]byte("u"), benchmarkBodySize)

	for _, workload := range []struct {
		name string
		path string
		body []byte
	}{
		{name: "upload-1MiB", path: "/upload", body: body},
		{name: "download-1MiB", path: "/download"},
	} {
		b.Run(workload.name, func(b *testing.B) {
			before := sampleProcess()
			b.ReportAllocs()
			b.SetBytes(benchmarkBodySize)
			perform := func() error {
				response, err := h.request(context.Background(), app.Id, workload.path, workload.body)
				if err != nil {
					return err
				}
				defer response.Body.Close()
				if response.StatusCode != http.StatusOK {
					return fmt.Errorf("unexpected status: %d", response.StatusCode)
				}
				_, err = io.Copy(io.Discard, response.Body)
				return err
			}
			if err := perform(); err != nil {
				b.Fatal(err)
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := perform(); err != nil {
					b.Fatal(err)
				}
			}
			b.StopTimer()
			reportProcessDelta(b, before)
		})
	}
}

func BenchmarkAshrixConcurrentRequests(b *testing.B) {
	app, _ := newTestApp(b, "ashrix-concurrent-benchmark-app")
	h := newProxyHarness(b, []*proto.ConnectorApps{app})
	body := bytes.Repeat([]byte("c"), 16<<10)

	for _, parallelism := range []int{1, 4, 16} {
		b.Run(fmt.Sprintf("parallelism-%d", parallelism), func(b *testing.B) {
			before := sampleProcess()
			b.ReportAllocs()
			b.SetBytes(int64(len(body) * 2))
			b.SetParallelism(parallelism)
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					response, err := h.request(context.Background(), app.Id, "/echo", body)
					if err != nil {
						b.Error(err)
						continue
					}
					_, err = io.Copy(io.Discard, response.Body)
					response.Body.Close()
					if err != nil {
						b.Error(err)
					}
				}
			})
			b.StopTimer()
			reportProcessDelta(b, before)
		})
	}
}

func BenchmarkAshrixConcurrentQUICStreams(b *testing.B) {
	app, _ := newTestApp(b, "ashrix-stream-benchmark-app")
	h := newProxyHarness(b, []*proto.ConnectorApps{app})
	tunnelSession, ok := h.registry.GetTunnelSession(testConnectorID)
	if !ok {
		b.Fatal("connector tunnel is not attached")
	}
	body := bytes.Repeat([]byte("s"), 16<<10)
	var requestID atomic.Uint64

	for _, parallelism := range []int{1, 4, 16} {
		b.Run(fmt.Sprintf("parallelism-%d", parallelism), func(b *testing.B) {
			before := sampleProcess()
			b.ReportAllocs()
			b.SetBytes(int64(len(body) * 2))
			b.SetParallelism(parallelism)
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
					err := benchmarkQUICStream(ctx, tunnelSession, app.Id, requestID.Add(1), body)
					cancel()
					if err != nil {
						b.Error(err)
					}
				}
			})
			b.StopTimer()
			reportProcessDelta(b, before)
		})
	}
}

func benchmarkHTTPRoundTrip(client *http.Client, url string, body []byte) error {
	request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %d", response.StatusCode)
	}
	_, err = io.Copy(io.Discard, response.Body)
	return err
}

func benchmarkQUICStream(ctx context.Context, tunnel registry.TunnelSession, appID string, requestID uint64, body []byte) error {
	request := &proto.StreamFrame{
		RequestId:    fmt.Sprintf("bench-%d", requestID),
		FlowType:     proto.FlowType_USER_TO_APP,
		DestAppId:    appID,
		Method:       http.MethodPost,
		ProtocolType: proto.ProtocolType_PROTOCOL_TCP,
		Path:         "/echo",
		BodyLength:   int64(len(body)),
		StreamType:   proto.RequestType_HTTP_REQUEST,
	}
	stream, err := tunnel.OpenStream(ctx, request)
	if err != nil {
		return err
	}
	defer stream.Close()

	httpRequest, err := http.NewRequest(http.MethodPost, "http://benchmark.local/echo", bytes.NewReader(body))
	if err != nil {
		return err
	}
	if err := httpRequest.Write(stream); err != nil {
		return err
	}
	response, err := http.ReadResponse(bufio.NewReader(stream), httpRequest)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	_, err = io.Copy(io.Discard, response.Body)
	return err
}
