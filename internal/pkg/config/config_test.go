package config

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLoad(t *testing.T) {
	t.Setenv("AUTONOMOUS_TOKEN", strings.Repeat("x", MinimumTokenLength))
	t.Setenv("TELEGRAM_BOT_TOKEN", "EXAMPLE-DO-NOT-USE")
	t.Setenv("TELEGRAM_CHAT_ID", "42")

	cfg, err := Load()
	require.NoError(t, err)
	require.Equal(t, 10*time.Minute, cfg.TunnelTTL)

	for _, mutate := range []func(*Config){
		func(c *Config) { c.Token = "short" },
		func(c *Config) { c.TelegramToken = "" },
		func(c *Config) { c.TelegramChatID = 0 },
		func(c *Config) { c.TunnelTTL = 0 },
		func(c *Config) { c.TunnelTTL = 2 * time.Hour },
		func(c *Config) { c.StartupTimeout = 0 },
		func(c *Config) { c.RequestTTL = 0 },
		func(c *Config) { c.ListenAddress = "invalid" },
		func(c *Config) { c.ViewerAddress = "0.0.0.0:8081" },
		func(c *Config) { c.BrowserViewerURL = "file:///etc/passwd" },
		func(c *Config) { c.StatePath = "relative" },
		func(c *Config) { c.CloudflaredBinary = "relative" },
	} {
		invalid := cfg
		mutate(&invalid)
		require.Error(t, invalid.Validate())
	}

	t.Setenv("TELEGRAM_CHAT_ID", "invalid")

	_, err = Load()
	require.Error(t, err)
	t.Setenv("TELEGRAM_CHAT_ID", "0")

	_, err = Load()
	require.Error(t, err)
}

func TestValidateURL(t *testing.T) {
	for _, value := range []string{
		"http://browser:5900",
		"https://phone.example/mcp",
	} {
		require.NoError(t, ValidateURL(value))
	}

	for _, value := range []string{
		"",
		"://",
		"file:///etc/passwd",
		"http://",
		"http://user:pass@host",
		"https://host/?token=x",
		"https://host/#token",
	} {
		require.Error(t, ValidateURL(value))
	}
}
