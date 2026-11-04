package setup

import (
	"crypto/sha256"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInitPreservesOperatorConfig(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, "scripts"), 0o700))

	for _, name := range []string{"scripts/init.sh", ".env.example"} {
		data, err := os.ReadFile(filepath.Join("../..", name))
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(root, name), data, 0o600))
	}

	run := func() {
		t.Helper()
		command := exec.CommandContext(t.Context(), "bash", "scripts/init.sh")
		command.Dir = root
		command.Env = append(
			os.Environ(),
			"LOG_FILE="+filepath.Join(root, "init.log"),
		)
		output, err := command.CombinedOutput()
		require.NoError(t, err, string(output))
	}
	run()

	path := filepath.Join(root, ".env")
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	info, err := os.Stat(path)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())

	keys := map[string]string{}

	for line := range strings.SplitSeq(string(data), "\n") {
		key, value, _ := strings.Cut(line, "=")
		if key == "AUTONOMOUS_TOKEN" || key == "BROWSER_TOKEN" ||
			key == "MAILBOX_TOKEN" {
			require.Len(t, value, 64)
			keys[key] = value
		}
	}

	require.Len(t, keys, 3)
	require.NotEqual(t, keys["AUTONOMOUS_TOKEN"], keys["BROWSER_TOKEN"])
	require.NotEqual(t, keys["BROWSER_TOKEN"], keys["MAILBOX_TOKEN"])

	before := sha256.Sum256(data)

	run()

	after, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, before, sha256.Sum256(after))
}
