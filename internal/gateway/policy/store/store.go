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
	ErrNotFound      = errors.New("policy not found")
	ErrHashMismatch  = errors.New("policy state hash mismatch")
	ErrStaleSequence = errors.New("stale policy sequence")
	ErrSigInvalid = errors.New("signature verification failed")
)

const (
	bucketPolicies   = "policies"
	bucketTombstones = "tombstones"
	bucketSyncMeta   = "sync_meta"
	bucketSOCKS5Creds = "socks5_credentials"
)

type SOCKS5Credential struct {
	ConnectorID  string    `json:"connector_id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"password_hash"`
	CreatedAt    time.Time `json:"created_at"`
}

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
		for _, name := range []string{bucketPolicies, bucketTombstones, bucketSyncMeta, bucketSOCKS5Creds} {
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

func (s *BoltStore) SetSOCKS5Credential(ctx context.Context, cred *SOCKS5Credential) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketSOCKS5Creds))
		data, err := json.Marshal(cred)
		if err != nil {
			return err
		}
		if err := b.Put([]byte("conn:"+cred.ConnectorID), data); err != nil {
			return err
		}
		return b.Put([]byte("user:"+cred.Username), data)
	})
}

func (s *BoltStore) GetSOCKS5CredentialByConnectorID(ctx context.Context, connectorID string) (*SOCKS5Credential, error) {
	var cred SOCKS5Credential
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketSOCKS5Creds))
		v := b.Get([]byte("conn:" + connectorID))
		if v == nil {
			return ErrNotFound
		}
		return json.Unmarshal(v, &cred)
	})
	if err != nil {
		return nil, err
	}
	return &cred, nil
}

func (s *BoltStore) GetSOCKS5CredentialByUsername(ctx context.Context, username string) (*SOCKS5Credential, error) {
	var cred SOCKS5Credential
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketSOCKS5Creds))
		v := b.Get([]byte("user:" + username))
		if v == nil {
			return ErrNotFound
		}
		return json.Unmarshal(v, &cred)
	})
	if err != nil {
		return nil, err
	}
	return &cred, nil
}

func (s *BoltStore) DeleteSOCKS5Credential(ctx context.Context, connectorID string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketSOCKS5Creds))
		v := b.Get([]byte("conn:" + connectorID))
		if v == nil {
			return nil
		}
		var cred SOCKS5Credential
		if err := json.Unmarshal(v, &cred); err == nil {
			_ = b.Delete([]byte("user:" + cred.Username))
		}
		return b.Delete([]byte("conn:" + connectorID))
	})
}

func (s *BoltStore) ListSOCKS5Credentials(ctx context.Context) ([]*SOCKS5Credential, error) {
	var out []*SOCKS5Credential
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketSOCKS5Creds))
		return b.ForEach(func(k, v []byte) error {
			if len(k) > 5 && string(k[:5]) == "conn:" {
				cred := new(SOCKS5Credential)
				if err := json.Unmarshal(v, cred); err == nil {
					out = append(out, cred)
				}
			}
			return nil
		})
	})
	return out, err
}

func (s *BoltStore) Close() error { return s.db.Close() }

func policyKey(tenantID, policyID string) []byte {
	return []byte(tenantID + "/" + policyID)
}

// LoadAllRecords returns full records including signatures. Used during bootstrap verification.
func (s *BoltStore) LoadAllRecords(ctx context.Context) ([]*pb.PolicyRecord, error) {
	var out []*pb.PolicyRecord
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketPolicies))
		return b.ForEach(func(k, v []byte) error {
			rec := new(pb.PolicyRecord)
			if err := proto.Unmarshal(v, rec); err != nil {
				return fmt.Errorf("decode %s: %w", k, err)
			}
			out = append(out, rec)
			return nil
		})
	})
	return out, err
}

// GetRecord returns a single policy with its signature and metadata.
func (s *BoltStore) GetRecord(ctx context.Context, id string) (*pb.PolicyRecord, error) {
	var rec *pb.PolicyRecord
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketPolicies))
		v := b.Get([]byte(id))
		if v == nil {
			return ErrNotFound
		}
		rec = new(pb.PolicyRecord)
		if err := proto.Unmarshal(v, rec); err != nil {
			return err
		}
		return nil
	})
	return rec, err
}

// // GetRecord returns a single verified policy.
// func (s *BoltStore) GetRecord(ctx context.Context, tenantID, policyID string) (*pb.PolicyRecord, error) {
// 	var rec *pb.PolicyRecord
// 	err := s.db.View(func(tx *bbolt.Tx) error {
// 		b := tx.Bucket([]byte(bucketPolicies))
// 		v := b.Get(policyKey(tenantID, policyID))
// 		if v == nil {
// 			return ErrNotFound
// 		}
// 		rec = new(pb.PolicyRecord)
// 		if err := proto.Unmarshal(v, rec); err != nil {
// 			return err
// 		}
// 		return s.verify(rec)
// 	})
// 	return rec, err
// }

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
func (s *BoltStore) ApplyDelta(ctx context.Context, policyRecords []*pb.PolicyRecord, checkpoint SyncCheckpoint) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		polB := tx.Bucket([]byte(bucketPolicies))
		tombB := tx.Bucket([]byte(bucketTombstones))
		metaB := tx.Bucket([]byte(bucketSyncMeta))

		// Upserts
		for _, record := range policyRecords {
			//If operation is upsert
			// Defensive: every record must carry at least policy_id + tenant_id
			if record.Rule == nil || record.Rule.PolicyId == "" {
				return fmt.Errorf("record missing policy_id")
			}

			key := policyKey(record.Rule.TenantId, record.Rule.PolicyId)

			// 2. Replay / rollback protection: sequence must increase
			if v := polB.Get(key); v != nil {
				existing := new(pb.PolicyRecord)
				if err := proto.Unmarshal(v, existing); err != nil {
					return fmt.Errorf("decode existing %s: %w", key, err)
				}
				if record.Sequence <= existing.Sequence {
					return fmt.Errorf("policy %s: %w (stored=%d, incoming=%d)",
						key, ErrStaleSequence, existing.Sequence, record.Sequence)
				}
			}
			if v := tombB.Get(key); v != nil {
				var tomb TombstoneRecord
				if err := json.Unmarshal(v, &tomb); err == nil {
					if record.Sequence <= tomb.Version {
						return fmt.Errorf("policy %s: %w (deleted at sequence %d, incoming=%d)",
							key, ErrStaleSequence, tomb.Version, record.Sequence)
					}
				}
			}
			// 3. Apply operation
			switch record.Operation {
			case pb.OperationEnum_OPERATION_ENUM_UPSERT:
				data, err := proto.Marshal(record)
				if err != nil {
					return fmt.Errorf("marshal %s: %w", key, err)
				}
				if err := polB.Put(key, data); err != nil {
					return err
				}

			case pb.OperationEnum_OPERATION_ENUM_DELETE:
				// Remove active policy
				if err := polB.Delete(key); err != nil {
					return err
				}
				// Tombstone for delta sync
				tomb := TombstoneRecord{
					ID:        record.Rule.PolicyId,
					DeletedAt: time.Now().UTC(),
					Version:   record.Sequence,
				}
				data, _ := json.Marshal(tomb)
				if err := tombB.Put(key, data); err != nil {
					return err
				}

			default:
				return fmt.Errorf("unknown operation %v for %s", record.Operation, key)

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
// Admin / Debug
// ---------------------------------------------------------------------

// DeleteAll wipes every policy (use with caution).
func (s *BoltStore) DeleteAll(ctx context.Context) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		if err := tx.DeleteBucket([]byte(bucketPolicies)); err != nil {
			return err
		}
		_, err := tx.CreateBucket([]byte(bucketPolicies))
		return err
	})
}
