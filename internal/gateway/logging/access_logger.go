package logging

import (
	"context"
	// "fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/config"
	pb "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gopkg.in/natefinch/lumberjack.v2"
)

const (
	logQueueSize  = 1024
	batchSize     = 50
	flushInterval = 5 * time.Second
)

type LogSender interface {
	SendLogBatch(batch *pb.AccessLogBatch) error
}

type AccessLogger struct {
	fileLogger *slog.Logger
	cfg        *config.Config

	// CP sender. This can change when the CP connection changes.
	mu     sync.RWMutex
	sender LogSender

	// Access logs waiting to be sent to CP.
	queue chan *pb.AccessLogEntry

	// CP streaming lifecycle.
	sessionMu sync.Mutex
	session   *cpSession

	// Prevent multiple Close calls.
	closeOnce sync.Once
	closed    chan struct{}

	// Observability.
	droppedLogs atomic.Uint64
}

type cpSession struct {
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func NewAccessLogger(cfg *config.Config, sender LogSender) *AccessLogger {
	if cfg.LogDir != "" {
		_ = os.MkdirAll(cfg.LogDir, 0750)
	}

	accessWriter := &lumberjack.Logger{
		Filename:   filepath.Join(cfg.LogDir, "access.log"),
		MaxSize:    100, // MB
		MaxAge:     30,  // days
		MaxBackups: 10,
		Compress:   true,
	}

	opts := &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}
	fileHandler := slog.NewJSONHandler(accessWriter, opts)
	fileLogger := slog.New(fileHandler).With("service", "gateway_access")

	return &AccessLogger{
		fileLogger: fileLogger,
		cfg:        cfg,
		sender:     sender,
		queue:      make(chan *pb.AccessLogEntry, 1024),
		closed:     make(chan struct{}),
	}
}

func (al *AccessLogger) SetSender(sender LogSender) {
	al.mu.Lock()
	defer al.mu.Unlock()
	al.sender = sender
}

func (al *AccessLogger) getSender() LogSender {
	al.mu.RLock()
	defer al.mu.RUnlock()

	return al.sender
}

func (al *AccessLogger) Log(ctx context.Context, entry *pb.AccessLogEntry, attrs ...any) {
	// 1. Write locally to access.log with file rotation
	if al.fileLogger != nil {
		al.fileLogger.InfoContext(ctx, "access_log", attrs...)
	}

	// 2. If log_to_cp is enabled, non-blocking enqueue for CP
	if entry == nil {
		return
	}

	select {
	case <-al.closed:
		return

	default:
	}

	// Never block the request path.
	select {
	case al.queue <- entry:
	default:
		// Queue full.
		//
		// We intentionally drop the log rather than slowing
		// down application traffic.
		al.droppedLogs.Add(1)
	}
}

func (al *AccessLogger) StartCPSync() {
	al.sessionMu.Lock()
	defer al.sessionMu.Unlock()

	// Already running.
	if al.session != nil {
		return
	}

	// No sender means we cannot send anything.
	if al.getSender() == nil {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())

	session := &cpSession{
		ctx:    ctx,
		cancel: cancel,
	}

	session.wg.Add(1)

	al.session = session

	go func() {
		defer session.wg.Done()

		al.flusherLoop(ctx)
	}()
}

func (al *AccessLogger) StopCPSync() {
	al.sessionMu.Lock()

	session := al.session
	al.session = nil

	al.sessionMu.Unlock()

	if session == nil {
		return
	}

	session.cancel()
	session.wg.Wait()
}

func (al *AccessLogger) flusherLoop(ctx context.Context) {
	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()

	batch := make(
		[]*pb.AccessLogEntry,
		0,
		batchSize,
	)

	flush := func() {
		if len(batch) == 0 {
			return
		}

		sender := al.getSender()

		if sender == nil {
			return
		}

		// IMPORTANT:
		// Don't reuse this slice after handing it to the sender.
		entries := batch

		err := sender.SendLogBatch(
			&pb.AccessLogBatch{
				Entries: entries,
			},
		)

		if err != nil {
			// The CP connection may have disappeared.
			//
			// We don't block the gateway waiting for CP.
			//
			// For the MVP, log the failure locally.
			if al.fileLogger != nil {
				al.fileLogger.Error(
					"failed to send access log batch",
					"error", err,
					"entries", len(entries),
				)
			}

			// Don't clear the batch on failure.
			//
			// We can retry it.
			return
		}

		// Successful send.
		batch = make(
			[]*pb.AccessLogEntry,
			0,
			batchSize,
		)
	}

	for {
		select {

		case <-ctx.Done():
			// Final flush for this CP session.
			flush()
			return

		case entry := <-al.queue:
			if entry == nil {
				continue
			}

			batch = append(batch, entry)

			if len(batch) >= batchSize {
				flush()
			}

		case <-ticker.C:
			flush()
		}
	}
}

func (al *AccessLogger) Close() {
	al.closeOnce.Do(func() {
		close(al.closed)

		al.StopCPSync()
	})
}

func (al *AccessLogger) DroppedLogs() uint64 {
	return al.droppedLogs.Load()
}


func BuildProtoAccessLogEntry(
	gatewayID string,
	tenantID string,
	appID string,
	userID string,
	userEmail string,
	method string,
	path string,
	status int32,
	latencyMs int32,
	clientIP string,
	Action string,
	policyID string,
	BytesIn int64,
	BytesOut int64,
	result string,
	denyReason string,
) *pb.AccessLogEntry {
	return &pb.AccessLogEntry{
		GatewayId:  gatewayID,
		OrgId:      tenantID,
		PolicyId:   policyID,
		AppId:      appID,
		UserId:     userID,
		UserEmail:  userEmail,
		Method:     method,
		Path:       path,
		Status:     status,
		LatencyMs:  latencyMs,
		Ip:         clientIP,
		Result:     result,
		Action:     Action,
		BytesIn:    BytesIn,
		BytesOut:   BytesOut,
		DenyReason: denyReason,
		CreatedAt:  timestamppb.Now(),
	}
}
