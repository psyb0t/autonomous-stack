package setup

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/psyb0t/ctxerrors/commerr"
	"github.com/stretchr/testify/require"
)

func TestPiBoxConfig(t *testing.T) {
	t.Parallel()

	cfg := validConfig()
	cfg.Profiles = "agent,browser"
	cfg.HarnessPlatform = piBox
	cfg.ZAIAuthToken = "your-z-ai-token-here"
	cfg.ZAIModel = "fixture-model"
	cfg.Effort = maximumEffort
	root := t.TempDir()

	require.NoError(t, Generate(cfg, root))
	mcpConfig, err := os.ReadFile(
		filepath.Join(root, "data/workspace/agent/.mcp.json"),
	)
	require.NoError(t, err)

	var project map[string]map[string]stdioMCPServer

	require.NoError(t, json.Unmarshal(mcpConfig, &project))
	require.Equal(t, stdioMCPServer{
		Command: "node",
		Args:    []string{"/opt/stack/pibox-mcp-bridge.js"},
		Env: map[string]string{
			"AUTONOMOUS_MCP_URL":           "http://browser:8080/mcp",
			"AUTONOMOUS_MCP_AUTHORIZATION": "Bearer " + cfg.BrowserToken,
		},
	}, project["mcpServers"]["browser"])

	cron, err := os.ReadFile(filepath.Join(root, "config/generated/cron.yml"))
	require.NoError(t, err)
	require.Contains(t, string(cron), `"schedule": "0 */30 * * * *"`)
	require.Contains(t, string(cron), `"effort": "xhigh"`)
	require.Contains(t, string(cron), `"model": "fixture-model"`)

	telegram, err := os.ReadFile(
		filepath.Join(root, "config/generated/telegram.yml"),
	)
	require.NoError(t, err)
	require.Contains(t, string(telegram), `"model": "fixture-model"`)

	cfg.HarnessPlatform = "claudebox"
	require.NoError(t, Generate(cfg, root))
	require.Equal(t, "http", readMCPFixture(t, root)["browser"].Type)
}

func TestHarnessValidation(t *testing.T) {
	t.Parallel()

	cfg := validConfig()
	cfg.Profiles = "agent"
	cfg.HarnessPlatform = "codexbox"
	require.ErrorIs(t, cfg.Validate(), commerr.ErrValidationFailed)

	cfg.HarnessPlatform = piBox
	require.ErrorIs(t, cfg.Validate(), commerr.ErrValidationFailed)

	cfg.ZAIAuthToken = "your-z-ai-token-here"
	cfg.ZAIModel = "fixture-model"
	require.NoError(t, cfg.Validate())
}
