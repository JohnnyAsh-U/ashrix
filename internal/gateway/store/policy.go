package store

// import (
// 	"context"
// 	"encoding/json"
// 	"errors"
// 	"fmt"
// 	"time"

// 	bolt "go.etcd.io/bbolt"
// )

// var (
// 	ErrNotFound = errors.New("policy not found")
// )

// // Store persists policies durably. Implementations: BoltStore, SQLiteStore.
// type Store interface {
// 	LoadAll(ctx context.Context) ([]Policy, error)
// 	Get(ctx context.Context, id string) (Policy, error)
// 	Put(ctx context.Context, p Policy) error
// 	Delete(ctx context.Context, id string) error
// 	Close() error
// }

// // BoltStore uses bbolt (pure Go, single file). Ideal for gateway edge nodes.
// type BoltStore struct {
// 	db *bolt.DB
// }

// const (
// 	bucketPolicies = "policies"
// 	bucketMeta     = "meta"
// 	keyGlobalVer   = "global_version"
// )

// // OpenBoltStore creates or opens the DB. Path should be inside gateway data dir.
// func OpenBoltStore(path string) (*BoltStore, error) {
// 	db, err := bolt.Open(path, 0600, &bolt.Options{Timeout: 5 * time.Second})
// 	if err != nil {
// 		return nil, fmt.Errorf("open bolt: %w", err)
// 	}

// 	if err := db.Update(func(tx *bolt.Tx) error {
// 		if _, err := tx.CreateBucketIfNotExists([]byte(bucketPolicies)); err != nil {
// 			return err
// 		}
// 		if _, err := tx.CreateBucketIfNotExists([]byte(bucketMeta)); err != nil {
// 			return err
// 		}
// 		return nil
// 	}); err != nil {
// 		return nil, fmt.Errorf("init bolt buckets: %w", err)
// 	}

// 	return &BoltStore{db: db}, nil
// }

// func (s *BoltStore) LoadAll(ctx context.Context) ([]Policy, error) {
// 	var out []Policy
// 	err := s.db.View(func(tx *bolt.Tx) error {
// 		b := tx.Bucket([]byte(bucketPolicies))
// 		if b == nil {
// 			return nil
// 		}
// 		return b.ForEach(func(k, v []byte) error {
// 			var p Policy
// 			if err := json.Unmarshal(v, &p); err != nil {
// 				return fmt.Errorf("unmarshal policy %s: %w", k, err)
// 			}
// 			out = append(out, p)
// 			return nil
// 		})
// 	})
// 	if err != nil {
// 		return nil, err
// 	}
// 	return out, nil
// }

// func (s *BoltStore) Get(ctx context.Context, id string) (Policy, error) {
// 	var p Policy
// 	err := s.db.View(func(tx *bolt.Tx) error {
// 		b := tx.Bucket([]byte(bucketPolicies))
// 		if b == nil {
// 			return ErrNotFound
// 		}
// 		v := b.Get([]byte(id))
// 		if v == nil {
// 			return ErrNotFound
// 		}
// 		return json.Unmarshal(v, &p)
// 	})
// 	if err != nil {
// 		return Policy{}, err
// 	}
// 	return p, nil
// }

// func (s *BoltStore) Put(ctx context.Context, p Policy) error {
// 	data, err := json.Marshal(p)
// 	if err != nil {
// 		return err
// 	}
// 	return s.db.Update(func(tx *bolt.Tx) error {
// 		b := tx.Bucket([]byte(bucketPolicies))
// 		if err := b.Put([]byte(p.ID), data); err != nil {
// 			return err
// 		}
// 		// bump global version for optimistic cache invalidation
// 		meta := tx.Bucket([]byte(bucketMeta))
// 		var gv int64
// 		if v := meta.Get([]byte(keyGlobalVer)); v != nil {
// 			_, _ = fmt.Sscanf(string(v), "%d", &gv)
// 		}
// 		gv++
// 		return meta.Put([]byte(keyGlobalVer), []byte(fmt.Sprintf("%d", gv)))
// 	})
// }

// func (s *BoltStore) Delete(ctx context.Context, id string) error {
// 	return s.db.Update(func(tx *bolt.Tx) error {
// 		b := tx.Bucket([]byte(bucketPolicies))
// 		if err := b.Delete([]byte(id)); err != nil {
// 			return err
// 		}
// 		meta := tx.Bucket([]byte(bucketMeta))
// 		var gv int64
// 		if v := meta.Get([]byte(keyGlobalVer)); v != nil {
// 			_, _ = fmt.Sscanf(string(v), "%d", &gv)
// 		}
// 		gv++
// 		return meta.Put([]byte(keyGlobalVer), []byte(fmt.Sprintf("%d", gv)))
// 	})
// }

// func (s *BoltStore) GlobalVersion(ctx context.Context) (int64, error) {
// 	var gv int64
// 	err := s.db.View(func(tx *bolt.Tx) error {
// 		meta := tx.Bucket([]byte(bucketMeta))
// 		v := meta.Get([]byte(keyGlobalVer))
// 		if v != nil {
// 			_, _ = fmt.Sscanf(string(v), "%d", &gv)
// 		}
// 		return nil
// 	})
// 	return gv, err
// }

// func (s *BoltStore) Close() error { return s.db.Close() }