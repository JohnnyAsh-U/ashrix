package crypto

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadControlPlanePKIDoesNotCreateMissingDirectory(t *testing.T) {
	baseDir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	_, err := LoadControlPlanePKI(baseDir, "test-secret", nil, nil, logger)

	require.Error(t, err)
	require.Contains(t, err.Error(), "ashrix-cp setup cp-cert")
	_, statErr := os.Stat(filepath.Join(baseDir, "pki", "cp"))
	require.True(t, os.IsNotExist(statErr))
}
