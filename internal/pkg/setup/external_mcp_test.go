package setup

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/psyb0t/ctxerrors/commerr"
	"github.com/stretchr/testify/require"
)

func externalMCPTestConfig() Config {
	cfg := validConfig()
	cfg.ModemBoxURL = "https://modem.example/mcp"
	cfg.ModemBoxToken = strings.Repeat("m", 32)
	cfg.ModemBoxModemID = "modem-123"
	cfg.PhoneNumber = "+12025550101"
	cfg.KeePassBoxURL = "https://vault.example/mcp"
	cfg.KeePassBoxToken = strings.Repeat("v", 32)

	return cfg
}

func TestGenerateExternalMCP(t *testing.T) {
	t.Parallel()

	cfg := externalMCPTestConfig()
	root := t.TempDir()

	require.NoError(t, Generate(cfg, root))
	servers := readMCPFixture(t, root)
	require.Equal(t, mcpServer{
		Type: "http",
		URL:  cfg.ModemBoxURL,
		Headers: map[string]string{
			"Authorization": "Bearer " + cfg.ModemBoxToken,
		},
	}, servers["modembox"])
	require.Equal(t, mcpServer{
		Type: "http",
		URL:  cfg.KeePassBoxURL,
		Headers: map[string]string{
			"Authorization": "Bearer " + cfg.KeePassBoxToken,
		},
	}, servers["keepassbox"])

	identity, err := os.ReadFile(
		filepath.Join(root, "config/generated/identity.json"),
	)
	require.NoError(t, err)
	require.Contains(t, string(identity), cfg.ModemBoxModemID)
	require.NotContains(t, string(identity), cfg.ModemBoxToken)
	require.NotContains(t, string(identity), cfg.KeePassBoxToken)

	require.NoError(t, Generate(validConfig(), root))
	servers = readMCPFixture(t, root)
	require.NotContains(t, servers, "modembox")
	require.NotContains(t, servers, "keepassbox")
}

func readMCPFixture(t *testing.T, root string) map[string]mcpServer {
	t.Helper()

	data, err := os.ReadFile(
		filepath.Join(root, "data/workspace/agent/.mcp.json"),
	)
	require.NoError(t, err)

	var config map[string]map[string]mcpServer

	require.NoError(t, json.Unmarshal(data, &config))

	return config["mcpServers"]
}

func TestValidateExternalMCP(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name   string
		mutate func(*Config)
	}{
		{"missing URL", func(c *Config) { c.ModemBoxURL = "" }},
		{"missing token", func(c *Config) { c.ModemBoxToken = "" }},
		{"invalid modem ID", func(c *Config) { c.ModemBoxModemID = "a/b" }},
		{"missing phone number", func(c *Config) { c.PhoneNumber = "" }},
		{"query", func(c *Config) { c.KeePassBoxURL += "?token=placeholder" }},
		{"invalid header", func(c *Config) { c.KeePassBoxToken += "\r\n" }},
		{"short token", func(c *Config) { c.KeePassBoxToken = "short" }},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cfg := externalMCPTestConfig()
			tc.mutate(&cfg)

			require.ErrorIs(t, cfg.Validate(), commerr.ErrValidationFailed)
		})
	}
}
