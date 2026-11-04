package setup

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func validConfig() Config {
	return Config{
		TailnetHostname: "autonomous-stack",
		TelegramChatID:  42,
		TelegramUserID:  84,
		TelegramToken:   "EXAMPLE-DO-NOT-USE",
		Model:           "sonnet",
		Effort:          "high",
		Schedule:        "*/30 * * * *",
		Heartbeat:       "Check tasks\nthen mail",
		SystemPrompt:    "Owner instructions",
		HandoffToken:    strings.Repeat("h", 32),
		BrowserToken:    strings.Repeat("b", 32),
		MailboxToken:    strings.Repeat("m", 32),
		IMAPHost:        "imap.example",
		IMAPPort:        993,
		IMAPTLS:         "ssl",
		SMTPHost:        "smtp.example",
		SMTPPort:        587,
		SMTPTLS:         "starttls",
		MailUsername:    "me@example.com",
		MailPassword:    "EXAMPLE-DO-NOT-USE",
		MailFrom:        "me@example.com",
	}
}

func TestGenerate(t *testing.T) {
	cfg := validConfig()
	root := t.TempDir()

	for _, phone := range []string{"", "https://phone.example/mcp"} {
		cfg.PhoneURL = phone
		if phone != "" {
			cfg.PhoneToken = "EXAMPLE-DO-NOT-USE"
		}

		require.NoError(t, Generate(cfg, root))

		for _, name := range []string{
			"config/generated/cron.yml",
			"config/generated/telegram.yml",
			"config/generated/mailbox.yml",
			"data/workspace/agent/.mcp.json",
			"data/workspace/agent/.claude/settings.local.json",
		} {
			path := filepath.Join(root, name)
			data, err := os.ReadFile(path)
			require.NoError(t, err)
			require.True(t, json.Valid(data))

			info, err := os.Stat(path)
			require.NoError(t, err)
			require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
		}

		data, err := os.ReadFile(
			filepath.Join(root, "data/workspace/agent/.mcp.json"),
		)
		require.NoError(t, err)

		var servers struct {
			//nolint:tagliatelle // Claude's project schema uses camel case.
			Servers map[string]mcpServer `json:"mcpServers"`
		}
		require.NoError(t, json.Unmarshal(data, &servers))
		require.Equal(
			t,
			"Bearer "+cfg.BrowserToken,
			servers.Servers["browser"].Headers["Authorization"],
		)
		_, enabled := servers.Servers["phone"]
		require.Equal(t, phone != "", enabled)
		require.NoError(t, Generate(cfg, root))
	}

	require.Error(t, Generate(Config{}, root))
	file := filepath.Join(root, "blocked")
	require.NoError(t, os.WriteFile(file, []byte("occupied"), 0o600))
	require.Error(t, Generate(cfg, file))
}

func TestValidate(t *testing.T) {
	for _, mutate := range []func(*Config){
		func(c *Config) { c.TailnetHostname = "invalid/name" },
		func(c *Config) { c.TelegramChatID = 0 },
		func(c *Config) { c.TelegramUserID = 0 },
		func(c *Config) { c.HandoffToken = "short" },
		func(c *Config) { c.Schedule = "* *" },
		func(c *Config) { c.Heartbeat = "" },
		func(c *Config) { c.Effort = "unknown" },
		func(c *Config) { c.SMTPPort = -1 },
		func(c *Config) { c.IMAPTLS = "none" },
		func(c *Config) { c.MailFrom = "invalid" },
		func(c *Config) { c.MailPassword = "" },
		func(c *Config) { c.PhoneURL = "file:///bad" },
		func(c *Config) { c.PhoneURL = "https://phone.example/mcp" },
		func(c *Config) { c.PhoneToken = "orphan" },
	} {
		cfg := validConfig()
		mutate(&cfg)
		require.Error(t, cfg.Validate())
	}

	_, err := Load()
	require.Error(t, err)
}
