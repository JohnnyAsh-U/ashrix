package store

import (
	"time"

	proto "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
)

// StoredPolicyRecord is what lives in bbolt under bucketPolicies.
// The Rule field is the proto.PolicyRule serialized to binary (base64 in JSON).
type StoredPolicyRecord struct {
	ID        string    `json:"id"`
	Version   int64     `json:"version"`     // gateway-local monotonic version
	Rule      []byte    `json:"rule"`        // proto.Marshal(PolicyRule)
	Signature []byte    `json:"signature"`   // Ed25519 signature over Rule bytes
	CPKeyID   string    `json:"cp_key_id"`   // key rotation identifier
	StoredAt  time.Time `json:"stored_at"`
}

// TombstoneRecord prevents deleted policies from being resurrected by stale deltas.
type TombstoneRecord struct {
	ID        string    `json:"id"`
	DeletedAt time.Time `json:"deleted_at"`
	Version   int64     `json:"version"` // last known version at deletion
}

// SyncCheckpoint anchors the gateway to the CP's version stream.
// The StateHash is a deterministic fingerprint of the live policy set.
type SyncCheckpoint struct {
	LastBundleVersion int64     `json:"last_bundle_version"`
	LastSyncAt        time.Time `json:"last_sync_at"`
	GatewayID         string    `json:"gateway_id"`
	TenantID          string    `json:"tenant_id"`
	StateHash         string    `json:"state_hash"`
	SignedByCP        bool      `json:"signed_by_cp"`
}

// PolicyRecord is the unmarshalled view returned by the store.
type PolicyRecord struct {
	ID        string
	Version   int64
	Rule      *proto.PolicyRule
	Signature []byte
	CPKeyID   string
	StoredAt  time.Time
}

// SyncDelta is what the CP sends (full snapshot or incremental).
type SyncDelta struct {
	IsFullSnapshot bool
	FromVersion    int64
	ToVersion      int64
	Upserts        []*proto.PolicyRule // new or changed rules
	DeleteIDs      []string            // rules to remove
	Signature      []byte              // Ed25519 over canonical delta bytes
	KeyID          string
}