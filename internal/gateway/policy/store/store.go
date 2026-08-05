package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"go.etcd.io/bbolt"
	bolt "go.etcd.io/bbolt"
	"google.golang.org/protobuf/proto"

	pb "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
)

var (
	ErrNotFound     = errors.New("policy not found")
	ErrHashMismatch = errors.New("policy state hash mismatch")
	ErrSigInvalid   = errors.New("signature verification failed")
)

const (
	bucketPolicies   = "policies"
	bucketTombstones = "tombstones"
	bucketSyncMeta   = "sync_meta"
)

// BoltStore persists policies durably. All writes are atomic bbolt transactions.
type BoltStore struct {
	db *bolt.DB
}

// OpenBoltStore creates or opens the database at path.
func OpenBoltStore(dataDir string) (*BoltStore, error) {
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return nil, fmt.Errorf("Create data dir: %w", err)
	}

	dbPath := filepath.Join(dataDir, "gateway.db")
	db, err := bbolt.Open(dbPath, 0600, &bbolt.Options{
		Timeout: 5 * time.Second,
	})

	if err != nil {
		return nil, fmt.Errorf("open bolt: %w", err)
	}

	if err := db.Update(func(tx *bolt.Tx) error {
		for _, name := range []string{bucketPolicies, bucketTombstones, bucketSyncMeta} {
			if _, err := tx.CreateBucketIfNotExists([]byte(name)); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("init buckets: %w", err)
	}

	return &BoltStore{db: db}, nil
}

func (s *BoltStore) Close() error { return s.db.Close() }

// ---------------------------------------------------------------------
// Reads
// ---------------------------------------------------------------------

// LoadAll returns all live policies (without signatures). Used to build engine index.
func (s *BoltStore) LoadAll(ctx context.Context) ([]*pb.PolicyRule, error) {
	var out []*pb.PolicyRule
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketPolicies))
		return b.ForEach(func(k, v []byte) error {
			var rec StoredPolicyRecord
			if err := json.Unmarshal(v, &rec); err != nil {
				return fmt.Errorf("unmarshal policy %s: %w", k, err)
			}
			rule, err := unmarshalRule(rec.Rule)
			if err != nil {
				return fmt.Errorf("unmarshal proto rule %s: %w", k, err)
			}
			out = append(out, rule)
			return nil
		})
	})
	return out, err
}

// LoadAllRecords returns full records including signatures. Used during bootstrap verification.
func (s *BoltStore) LoadAllRecords(ctx context.Context) ([]PolicyRecord, error) {
	var out []PolicyRecord
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketPolicies))
		return b.ForEach(func(k, v []byte) error {
			rec, err := decodeRecord(v)
			if err != nil {
				return fmt.Errorf("decode policy %s: %w", k, err)
			}
			out = append(out, *rec)
			return nil
		})
	})
	return out, err
}

// GetRecord returns a single policy with its signature and metadata.
func (s *BoltStore) GetRecord(ctx context.Context, id string) (*PolicyRecord, error) {
	var rec *PolicyRecord
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketPolicies))
		v := b.Get([]byte(id))
		if v == nil {
			return ErrNotFound
		}
		var err error
		rec, err = decodeRecord(v)
		return err
	})
	return rec, err
}

// GetCheckpoint returns the last sync checkpoint. Zero value if first boot.
func (s *BoltStore) GetCheckpoint(ctx context.Context) (SyncCheckpoint, error) {
	var cp SyncCheckpoint
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketSyncMeta))
		v := b.Get([]byte("checkpoint"))
		if v == nil {
			return nil
		}
		return json.Unmarshal(v, &cp)
	})
	return cp, err
}

// ListTombstones returns recently deleted policy IDs with deletion timestamps.
func (s *BoltStore) ListTombstones(ctx context.Context) (map[string]TombstoneRecord, error) {
	out := make(map[string]TombstoneRecord)
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketTombstones))
		return b.ForEach(func(k, v []byte) error {
			var t TombstoneRecord
			if err := json.Unmarshal(v, &t); err != nil {
				return err
			}
			out[string(k)] = t
			return nil
		})
	})
	return out, err
}

// ---------------------------------------------------------------------
// Atomic Writes
// ---------------------------------------------------------------------

// ApplyDelta performs all mutations and the checkpoint update inside a single
// bbolt transaction. Either everything commits or nothing does.
func (s *BoltStore) ApplyDelta(ctx context.Context, upserts []PolicyRecord, deleteIDs []string, checkpoint SyncCheckpoint) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		polB := tx.Bucket([]byte(bucketPolicies))
		tombB := tx.Bucket([]byte(bucketTombstones))
		metaB := tx.Bucket([]byte(bucketSyncMeta))

		// Upserts
		for _, pr := range upserts {
			ruleBytes, err := proto.Marshal(pr.Rule)
			if err != nil {
				return fmt.Errorf("marshal rule %s: %w", pr.ID, err)
			}
			rec := StoredPolicyRecord{
				ID:        pr.ID,
				Version:   pr.Version,
				Rule:      ruleBytes,
				Signature: pr.Signature,
				// CPKeyID:   pr.CPID,
				StoredAt: time.Now().UTC(),
			}
			data, err := json.Marshal(rec)
			if err != nil {
				return err
			}
			if err := polB.Put([]byte(pr.ID), data); err != nil {
				return err
			}
		}

		// Deletions (soft delete → tombstone)
		for _, id := range deleteIDs {
			// Fetch existing version for the tombstone
			var lastVersion int64
			if v := polB.Get([]byte(id)); v != nil {
				var rec StoredPolicyRecord
				_ = json.Unmarshal(v, &rec)
				lastVersion = rec.Version
			}

			if err := polB.Delete([]byte(id)); err != nil {
				return err
			}
			tomb := TombstoneRecord{
				ID:        id,
				DeletedAt: time.Now().UTC(),
				Version:   lastVersion,
			}
			data, _ := json.Marshal(tomb)
			if err := tombB.Put([]byte(id), data); err != nil {
				return err
			}
		}

		// Checkpoint
		cpData, err := json.Marshal(checkpoint)
		if err != nil {
			return err
		}
		return metaB.Put([]byte("checkpoint"), cpData)
	})
}

// ---------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------

func unmarshalRule(b []byte) (*pb.PolicyRule, error) {
	var r pb.PolicyRule
	if err := proto.Unmarshal(b, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

func decodeRecord(v []byte) (*PolicyRecord, error) {
	var rec StoredPolicyRecord
	if err := json.Unmarshal(v, &rec); err != nil {
		return nil, err
	}
	rule, err := unmarshalRule(rec.Rule)
	if err != nil {
		return nil, err
	}
	return &PolicyRecord{
		ID:        rec.ID,
		Version:   rec.Version,
		Rule:      rule,
		Signature: rec.Signature,
		CPKeyID:   rec.CPKeyID,
		StoredAt:  rec.StoredAt,
	}, nil
}
