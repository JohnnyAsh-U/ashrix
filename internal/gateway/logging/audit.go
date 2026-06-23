package logging

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"sync"
)

// internal/logger/audit.go

type AuditEvent struct {
    // Identity
    Timestamp   string `json:"ts"`
    EventType   string `json:"event"`          // CONN_REGISTER, ACCESS_GRANT, etc.
    GatewayID   string `json:"gateway_id"`
    TenantID    string `json:"tenant_id"`
    ConnectorID string `json:"connector_id,omitempty"`
    UserID      string `json:"user_id,omitempty"`
    UserEmail   string `json:"user_email,omitempty"`

    // Request context
    AppID       string `json:"app_id,omitempty"`
    Method      string `json:"method,omitempty"`
    Path        string `json:"path,omitempty"`
    StatusCode  int    `json:"status_code,omitempty"`
    LatencyMS   int64  `json:"latency_ms,omitempty"`

    // Decision
    Decision    string `json:"decision"`       // ALLOW, DENY
    DenyReason  string `json:"deny_reason,omitempty"`

    // Tamper evidence
    Sequence    int64  `json:"seq"`
    PrevHash    string `json:"prev_hash"`
    Hash        string `json:"hash"`
}

type auditLogger struct {
    mu       sync.Mutex
    file     *os.File
    sequence int64
    prevHash string
}

func newAuditLogger(file *os.File) *auditLogger {
    return &auditLogger{
        file:     file,
        prevHash: "genesis", // first entry chains to this
    }
}

func (a *auditLogger) Log(event AuditEvent) {
    a.mu.Lock()
    defer a.mu.Unlock()

    a.sequence++
    event.Sequence = a.sequence
    event.PrevHash = a.prevHash

    // Hash this entry (without the Hash field itself)
    event.Hash = ""
    data, _ := json.Marshal(event)
    h := sha256.Sum256(data)
    event.Hash = hex.EncodeToString(h[:])

    a.prevHash = event.Hash

    // Write to file — one JSON line per entry
    line, _ := json.Marshal(event)
    a.file.Write(append(line, '\n'))
}





