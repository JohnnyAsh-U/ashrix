package crypto

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBundleSigningLoadDoesNotProvisionMissingKey(t *testing.T) {
	baseDir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	keyPath := filepath.Join(baseDir, "pki", "cp", "bundle-signer.enc")

	_, err := BundleSigningKeys(baseDir, "test-secret", logger)

	require.Error(t, err)
	require.Contains(t, err.Error(), "ashrix-cp setup bundle-key")
	_, statErr := os.Stat(filepath.Dir(keyPath))
	require.True(t, os.IsNotExist(statErr))
}

func TestProvisionBundleSigningKeyCanBeLoadedAtRuntime(t *testing.T) {
	baseDir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	provisioned, err := ProvisionBundleSigningKeys(baseDir, "test-secret", logger)
	require.NoError(t, err)
	require.NotNil(t, provisioned)

	loaded, err := BundleSigningKeys(baseDir, "test-secret", logger)
	require.NoError(t, err)
	require.NotNil(t, loaded)
}
