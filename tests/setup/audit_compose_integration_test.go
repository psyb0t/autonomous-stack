package setup

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestComposePolicy(t *testing.T) {
	for _, scenario := range []string{
		"valid", "public-port", "docker-socket", "privileged",
		"capability", "writable-root", "missing-health", "unpinned-image",
		"invalid-tmpfs",
	} {
		t.Run(scenario, func(t *testing.T) {
			service := map[string]any{
				"image": "psyb0t/autonomous-stack:latest",
				"init":  true, "cap_drop": []string{"ALL"},
				"security_opt": []string{"no-new-privileges:true"},
				"read_only":    true, "mem_limit": 268435456,
				"cpus": 1, "pids_limit": 128,
				"healthcheck": map[string]any{
					"test": []string{"CMD", "healthcheck"},
				},
				"logging": map[string]any{"options": map[string]any{
					"max-size": "10m", "max-file": "5",
				}},
				"restart":  "unless-stopped",
				"networks": map[string]any{"stack": nil},
			}

			switch scenario {
			case "public-port":
				service["ports"] = []any{map[string]any{"host_ip": "0.0.0.0"}}
			case "docker-socket":
				service["volumes"] = []any{map[string]any{
					"type": "bind", "source": "/var/run/docker.sock",
				}}
			case "privileged":
				service["privileged"] = true
			case "capability":
				service["cap_add"] = []string{"SYS_ADMIN"}
			case "writable-root":
				service["read_only"] = false
			case "missing-health":
				delete(service, "healthcheck")
			case "unpinned-image":
				service["image"] = "outside/image:latest"
			case "invalid-tmpfs":
				service["tmpfs"] = []string{"/tmp:size=64m", "noexec", "nosuid"}
			}

			data, err := json.Marshal(map[string]any{
				"services": map[string]any{"handoff": service},
			})
			require.NoError(t, err)
			path := filepath.Join(t.TempDir(), "compose.json")
			require.NoError(t, os.WriteFile(path, data, 0o600))
			command := exec.CommandContext(
				t.Context(), "bash", "../../scripts/audit-compose.sh", path,
			)

			output, err := command.CombinedOutput()
			if scenario == "valid" {
				require.NoError(t, err, string(output))
			} else {
				require.Error(t, err)
			}
		})
	}
}
