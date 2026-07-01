package store

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"go.etcd.io/bbolt"
	"go.uber.org/zap"
)

// Bucket names all gateway state lives in these buckets
var bucketAccessLogs = []byte("pending_access_logs")


//Store is the gateway local persistent store.
// Hold only what needs to survive  when a process restart
//Source of truth for everything is the CP - this is a cache

type Store struct {
	db  *bbolt.DB
	log *zap.Logger
}

func Open(dataDir string, log *zap.Logger) (*Store, error) {
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return nil, fmt.Errorf("Create data dir: %w", err)
	}

	dbPath := filepath.Join(dataDir, "gateway.db")
	db, err := bbolt.Open(dbPath, 0600, &bbolt.Options{
		Timeout: 1 * time.Second,
	})

	if err != nil {
		return nil, fmt.Errorf("Open Store at %s: %w", dbPath, err)
	}

	s := &Store{db: db, log: log}

	if err := s.initBuckets(); err != nil {
		db.Close()
		return nil, fmt.Errorf("init buckets: %w", err)
	}

	log.Info("Store opened", zap.String("path", dbPath))
	return s, nil
}


//Close closes the store cleanly
//Flways defer this after Open.

func (s *Store) Close() error {
	return s.db.Close()
}


func (s *Store) initBuckets() error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		// for _, bucket := range [][]byte{bucketBundles, bucketConfig} {
			if _, err := tx.CreateBucketIfNotExists(bucketAccessLogs); err != nil {
				return fmt.Errorf("Create bucket %s: %w", bucketAccessLogs, err)
			}
		// }
		return nil
	})
}


func (s *Store) EnqueueAccessLog(entry struct{}) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketAccessLogs)
		data, err := json.Marshal(entry)

		if err != nil {
			return fmt.Errorf("Marshal log entry: %w", err)
		}

		seq, _ := b.NextSequence()
		key := seqToKey(seq)
		return b.Put(key, data)
	})
}

func (s *Store) MarkShipped(keys [][]byte) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketAccessLogs)
		for _, k := range keys {
			if err := b.Delete(k); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Store) PendingLogs(limit int) (keys [][]byte, entries []*struct{}, err error) {
	err = s.db.View(func(tx *bbolt.Tx) error{
		b := tx.Bucket(bucketAccessLogs)
		c := b.Cursor()

		count := 0
		for k, v := c.First(); k != nil && count < limit; k, v =c.Next(){
			var entry struct{}
			if err :=json.Unmarshal(v, &entry); err != nil {
				continue
			}
			keys = append(keys, append([]byte{}, k...))
			entries = append(entries, &entry)
			count++
		}
		return nil
	})
	return
}

func seqToKey(n uint64) []byte {
	b :=make([]byte, 8)
	binary.BigEndian.PutUint64(b, n)
	return b
}



