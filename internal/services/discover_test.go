package services

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDiscover(t *testing.T) {
	repoRoot := t.TempDir()
	cmdDir := filepath.Join(repoRoot, "cmd")
	require.NoError(t, os.MkdirAll(filepath.Join(cmdDir, "be-temporal-sync"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(cmdDir, "be-public-api"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(cmdDir, "misc"), 0o755))

	services, err := Discover(repoRoot)
	require.NoError(t, err)
	require.Len(t, services, 2)
	require.Equal(t, "be-public-api", services[0].Binary)
	require.Equal(t, "public_api", services[0].Service)
	require.Equal(t, filepath.Join("cmd", "be-public-api"), services[0].Path)
}

