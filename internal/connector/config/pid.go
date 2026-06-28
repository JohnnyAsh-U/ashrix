package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"github.com/gofrs/flock"
)

type Locker struct {
	fileLock *flock.Flock
	pidFile string
}


func NewPID(baseDir string) *Locker {
	return &Locker{
		fileLock: flock.New(filepath.Join(baseDir, "connector.lock")),
		pidFile: filepath.Join(baseDir, "connector.pid"),
	}
}

func (l *Locker) Acquire() error {
	locked, err := l.fileLock.TryLock()
	if err != nil {
		return fmt.Errorf("lock error: %w", err)
	}

	if !locked {
		return fmt.Errorf("another connector is already running")
	}

	return os.WriteFile(l.pidFile, []byte(strconv.Itoa(os.Getpid())), 0600)
}


func (l *Locker) Release(){
	l.fileLock.Unlock() //Releases OS Lock
	os.Remove(l.pidFile) //cleanup
}