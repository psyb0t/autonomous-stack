package handoff

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/psyb0t/autonomous-stack/internal/pkg/config"
	"github.com/stretchr/testify/require"
)

func TestStore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "state.json")
	_, err := loadState(path)
	require.NoError(t, err)

	state := State{
		ID:        "identifier",
		Key:       "task",
		Reason:    "\u9700\u8981\u5e2e\u52a9",
		Phase:     PhasePending,
		CreatedAt: time.Now().UTC(),
	}
	require.NoError(t, saveState(path, state))
	loaded, err := loadState(path)
	require.NoError(t, err)
	require.Equal(t, state, loaded)

	info, err := os.Stat(path)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())

	for _, value := range []string{
		"broken",
		`{ "phase":"unknown" }`,
		strings.Repeat("x",
			65537),
	} {
		require.NoError(t, os.WriteFile(path, []byte(value), 0o600))
		_, err := loadState(path)
		require.Error(t, err)
	}

	_, err = loadState(filepath.Dir(path))
	require.Error(t, err)
	require.Error(t, saveState(filepath.Join(path, "invalid"), state))
	require.NoError(t, os.Remove(path))
	require.NoError(t, os.Mkdir(path, 0o700))
	require.Error(t, saveState(path, state))
	_, err = New(
		config.Config{StatePath: path},
		&fixtureLauncher{},
		&fixtureNotifier{},
	)
	require.Error(t, err)
}
