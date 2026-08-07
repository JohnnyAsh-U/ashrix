package store

import (
	"time"
)

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
	// GatewayID         string    `json:"gateway_id"`
}
