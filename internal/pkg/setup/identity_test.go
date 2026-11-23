package setup

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPhoneNumberValidation(t *testing.T) {
	t.Parallel()

	for _, number := range []string{"", "+12", "+" + strings.Repeat("1", 15)} {
		cfg := validConfig()
		cfg.PhoneNumber = number
		require.NoError(t, cfg.Validate())
	}

	for _, number := range []string{
		"+", "12", "+01", "+1 2", "+１２", "+" + strings.Repeat("1", 16),
	} {
		cfg := validConfig()
		cfg.PhoneNumber = number
		require.Error(t, cfg.Validate())
	}
}

func TestIdentityAndPasswordPersistence(t *testing.T) {
	t.Parallel()

	cfg := validConfig()
	cfg.PhoneNumber = "+12025550123"
	root := t.TempDir()
	require.NoError(t, Generate(cfg, root))

	identityPath := filepath.Join(root, "config/generated/identity.json")
	data, err := os.ReadFile(identityPath)
	require.NoError(t, err)

	var identity struct {
		Email   string            `json:"email"`
		Phone   string            `json:"phone_number"`
		Mailbox map[string]string `json:"mailbox"`
	}
	require.NoError(t, json.Unmarshal(data, &identity))
	require.Equal(t, cfg.MailFrom, identity.Email)
	require.Equal(t, cfg.PhoneNumber, identity.Phone)
	require.Equal(t, cfg.MailPassword, identity.Mailbox["password"])

	info, err := os.Stat(identityPath)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())

	directory := filepath.Join(root, "data/workspace/agent/private")
	path := filepath.Join(directory, "passwords.json")
	data, err = os.ReadFile(path)
	require.NoError(t, err)
	require.JSONEq(t, `{"accounts":[]}`, string(data))

	info, err = os.Stat(directory)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o700), info.Mode().Perm())

	saved := []byte(
		`{"accounts":[{"password":"EXAMPLE-DO-NOT-USE"}]}`,
	)
	require.NoError(t, os.WriteFile(path, saved, 0o600))

	cfg.MailPassword = "changed-example-password"
	require.NoError(t, Generate(cfg, root))

	data, err = os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, saved, data)

	info, err = os.Stat(path)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestPasswordPathSafety(t *testing.T) {
	t.Parallel()

	for _, scenario := range []string{
		"symlink", "directory", "public-file", "public-directory",
		"directory-symlink",
	} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			directory := filepath.Join(root, "private")
			require.NoError(t, os.Mkdir(directory, 0o700))
			path := filepath.Join(directory, "passwords.json")
			target := filepath.Join(root, "untouched")
			require.NoError(t, os.WriteFile(target, []byte("keep"), 0o600))

			switch scenario {
			case "symlink":
				require.NoError(t, os.Symlink(target, path))
			case "directory":
				require.NoError(t, os.Mkdir(path, 0o700))
			case "public-file":
				require.NoError(t, os.WriteFile(path, []byte("keep"), 0o600))
				// #nosec G302. Test rejects public permissions.
				require.NoError(t, os.Chmod(path, 0o644))
			case "public-directory":
				// #nosec G302. Test rejects public permissions.
				require.NoError(t, os.Chmod(directory, 0o755))
			case "directory-symlink":
				directory = filepath.Join(root, "link")
				require.NoError(
					t,
					os.Symlink(filepath.Join(root, "private"), directory),
				)
			}

			require.Error(t, ensurePasswordFile(directory))

			data, err := os.ReadFile(target)
			require.NoError(t, err)
			require.Equal(t, "keep", string(data))
		})
	}
}

func TestConcurrentPasswordInitialization(t *testing.T) {
	t.Parallel()
	directory := filepath.Join(t.TempDir(), "private")

	var workers sync.WaitGroup

	results := make(chan error, 10)

	for range 10 {
		workers.Go(func() { results <- ensurePasswordFile(directory) })
	}

	workers.Wait()
	close(results)

	for err := range results {
		require.NoError(t, err)
	}

	data, err := os.ReadFile(filepath.Join(directory, "passwords.json"))
	require.NoError(t, err)
	require.JSONEq(t, `{"accounts":[]}`, string(data))
}
