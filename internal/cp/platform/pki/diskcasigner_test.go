package pki

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewDiskCASignerDoesNotCreateMissingPKIDirectory(t *testing.T) {
	baseDir := t.TempDir()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	_, err := NewDiskCASigner(baseDir, "test-secret", nil, log)

	require.Error(t, err)
	require.Contains(t, err.Error(), "ashrix-cp setup pki")
	_, statErr := os.Stat(filepath.Join(baseDir, "pki"))
	require.True(t, os.IsNotExist(statErr))
}
