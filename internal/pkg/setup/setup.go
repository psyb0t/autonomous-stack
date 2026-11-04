package setup

import (
	"encoding/json"
	"errors"
	"net/mail"
	"os"
	"path/filepath"
	"strings"

	"github.com/psyb0t/autonomous-stack/internal/pkg/config"
	"github.com/psyb0t/ctxerrors"
	"github.com/psyb0t/ctxerrors/commerr"
	"github.com/psyb0t/gonfiguration"
)

const privateDirectoryMode = 0o700

type Config struct {
	TelegramChatID  int64  `env:"TELEGRAM_CHAT_ID,required"`
	TelegramUserID  int64  `env:"TELEGRAM_USER_ID,required"`
	TelegramToken   string `env:"TELEGRAM_BOT_TOKEN,required"`
	Model           string `default:"sonnet"                  env:"CLAUDE_MODEL"`
	Effort          string `default:"high"                    env:"CLAUDE_EFFORT"`
	Schedule        string `default:"*/30 * * * *"            env:"HEARTBEAT_SCHEDULE"`
	Heartbeat       string `env:"HEARTBEAT_PROMPT,required"`
	SystemPrompt    string `env:"AGENT_PROMPT,required"`
	HandoffToken    string `env:"AUTONOMOUS_TOKEN,required"`
	BrowserToken    string `env:"BROWSER_TOKEN,required"`
	MailboxToken    string `env:"MAILBOX_TOKEN,required"`
	IMAPHost        string `env:"MAIL_IMAP_HOST,required"`
	IMAPPort        int    `default:"993"                     env:"MAIL_IMAP_PORT"`
	IMAPTLS         string `default:"ssl"                     env:"MAIL_IMAP_TLS"`
	SMTPHost        string `env:"MAIL_SMTP_HOST,required"`
	SMTPPort        int    `default:"587"                     env:"MAIL_SMTP_PORT"`
	SMTPTLS         string `default:"starttls"                env:"MAIL_SMTP_TLS"`
	MailUsername    string `env:"MAIL_USERNAME,required"`
	MailPassword    string `env:"MAIL_PASSWORD,required"`
	MailFrom        string `env:"MAIL_FROM,required"`
	MailSaveSent    bool   `default:"true"                    env:"MAIL_SAVE_SENT"`
	AuthservID      string `env:"MAIL_TRUSTED_AUTHSERV_ID"`
	PhoneURL        string `env:"DIKCIZ_MCP_URL"`
	PhoneToken      string `env:"DIKCIZ_API_KEY"`
	TailnetHostname string `default:"autonomous-stack"        env:"TS_HOSTNAME"`
}

func Load() (Config, error) {
	var cfg Config
	if err := gonfiguration.Parse(&cfg); err != nil {
		return Config{}, ctxerrors.Wrap(err, "parse stack setup")
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, ctxerrors.Wrap(err, "validate stack setup")
	}

	return cfg, nil
}

func (c Config) Validate() error {
	if err := config.ValidateHostname(c.TailnetHostname); err != nil {
		return ctxerrors.Wrap(err, "tailnet hostname")
	}

	if err := c.validateOwner(); err != nil {
		return ctxerrors.Wrap(err, "owner configuration")
	}

	if err := c.validateAgent(); err != nil {
		return ctxerrors.Wrap(err, "agent configuration")
	}

	if err := c.validateMail(); err != nil {
		return ctxerrors.Wrap(err, "mail configuration")
	}

	if err := c.validatePhone(); err != nil {
		return ctxerrors.Wrap(err, "phone configuration")
	}

	return nil
}

func (c Config) validateOwner() error {
	if c.TelegramChatID == 0 || c.TelegramUserID <= 0 ||
		strings.TrimSpace(c.TelegramToken) == "" {
		return ctxerrors.Wrap(
			commerr.ErrValidationFailed,
			"set the Telegram bot, owner chat and owner user ID",
		)
	}

	tokens := []string{c.HandoffToken, c.BrowserToken, c.MailboxToken}
	for _, token := range tokens {
		if len(strings.TrimSpace(token)) < config.MinimumTokenLength ||
			strings.Contains(token, "REPLACE_ME") {
			return ctxerrors.Wrap(
				commerr.ErrValidationFailed,
				"generate distinct internal API tokens with make init",
			)
		}
	}

	return nil
}

func (c Config) validateAgent() error {
	fields := strings.Fields(c.Schedule)
	if len(fields) != 5 && len(fields) != 6 {
		return ctxerrors.Wrap(
			commerr.ErrValidationFailed,
			"heartbeat schedule needs five or six cron fields",
		)
	}

	if strings.TrimSpace(c.Heartbeat) == "" ||
		strings.TrimSpace(c.SystemPrompt) == "" ||
		c.Model == "" {
		return ctxerrors.Wrap(
			commerr.ErrValidationFailed,
			"model and prompts cannot be empty",
		)
	}

	switch c.Effort {
	case "low", "medium", "high", "max":
	default:
		return ctxerrors.Wrap(
			commerr.ErrValidationFailed,
			"unsupported Claude effort",
		)
	}

	return nil
}

func (c Config) validateMail() error {
	if err := c.validateMailTransport(); err != nil {
		return ctxerrors.Wrap(err, "mail transport")
	}

	if _, err := mail.ParseAddress(c.MailFrom); err != nil {
		return ctxerrors.Wrap(err, "invalid mail sender")
	}

	if c.IMAPHost == "" || c.SMTPHost == "" || c.MailUsername == "" ||
		c.MailPassword == "" {
		return ctxerrors.Wrap(
			commerr.ErrValidationFailed,
			"mail account settings are required",
		)
	}

	return nil
}

func (c Config) validatePhone() error {
	if c.PhoneURL != "" {
		if err := config.ValidateURL(c.PhoneURL); err != nil {
			return ctxerrors.Wrap(err, "invalid Dikciz MCP URL")
		}

		if c.PhoneToken == "" {
			return ctxerrors.Wrap(
				commerr.ErrValidationFailed,
				"Dikciz API key is required with its URL",
			)
		}
	} else if c.PhoneToken != "" {
		return ctxerrors.Wrap(
			commerr.ErrValidationFailed,
			"Dikciz URL is required with its API key",
		)
	}

	return nil
}

type mcpServer struct {
	Type    string            `json:"type"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
}

func Generate(cfg Config, root string) error {
	if err := cfg.Validate(); err != nil {
		return ctxerrors.Wrap(err, "refuse invalid generated configuration")
	}

	cron, telegram := cronConfig(cfg), telegramConfig(cfg)

	servers := map[string]mcpServer{
		"browser": {
			Type: "http",
			URL:  "http://browser:8080/mcp",
			Headers: map[string]string{
				"Authorization": "Bearer " + cfg.BrowserToken,
			},
		},
		"mailbox": {
			Type: "http",
			URL:  "http://mailbox:8000/mcp",
			Headers: map[string]string{
				"Authorization": "Bearer " + cfg.MailboxToken,
			},
		},
		"handoff": {
			Type: "http",
			URL:  "http://handoff:8080/mcp",
			Headers: map[string]string{
				"Authorization": "Bearer " + cfg.HandoffToken,
			},
		},
	}
	if cfg.PhoneURL != "" {
		servers["phone"] = mcpServer{
			Type: "http",
			URL:  cfg.PhoneURL,
			Headers: map[string]string{
				"Authorization": "Bearer " + cfg.PhoneToken,
			},
		}
	}

	mailbox := mailboxConfig(cfg)

	files := map[string]any{
		"config/generated/cron.yml":     cron,
		"config/generated/telegram.yml": telegram,
		"config/generated/mailbox.yml":  mailbox,
		"data/workspace/agent/.mcp.json": map[string]any{
			"mcpServers": servers,
		},
		"data/workspace/agent/.claude/settings.local.json": map[string]any{
			"enableAllProjectMcpServers": true,
		},
	}
	for name, value := range files {
		if err := writeJSON(filepath.Join(root, name), value); err != nil {
			return ctxerrors.Wrap(err, "write private runtime configuration")
		}
	}

	return nil
}

func writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return ctxerrors.Wrap(err, "encode configuration")
	}

	if err := os.MkdirAll(
		filepath.Dir(path),
		privateDirectoryMode,
	); err != nil {
		return ctxerrors.Wrap(err, "create configuration directory")
	}

	file, err := os.CreateTemp(filepath.Dir(path), ".setup-*")
	if err != nil {
		return ctxerrors.Wrap(err, "create configuration file")
	}

	name := file.Name()

	// The renamed file no longer exists at this temporary path.
	defer func() { _ = os.Remove(name) }()

	if _, err := file.Write(data); err != nil {
		closeErr := file.Close()

		return ctxerrors.Wrap(errors.Join(err, closeErr), "write configuration")
	}

	if err := file.Close(); err != nil {
		return ctxerrors.Wrap(err, "close configuration")
	}

	if err := os.Rename(name, path); err != nil {
		return ctxerrors.Wrap(err, "replace generated configuration")
	}

	return nil
}

func cronConfig(cfg Config) map[string]any {
	return map[string]any{
		"model":                cfg.Model,
		"effort":               cfg.Effort,
		"telegram_chat_id":     cfg.TelegramChatID,
		"append_system_prompt": cfg.SystemPrompt,
		"jobs": []any{
			map[string]any{
				"name":        "heartbeat",
				"schedule":    cfg.Schedule,
				"workspace":   "agent",
				"instruction": cfg.Heartbeat,
			},
		},
	}
}

func telegramConfig(cfg Config) map[string]any {
	return map[string]any{
		"allowed_chats": []int64{cfg.TelegramChatID},
		"default": map[string]any{
			"model":                cfg.Model,
			"effort":               cfg.Effort,
			"continue":             true,
			"append_system_prompt": cfg.SystemPrompt,
		},
		"chats": map[int64]any{
			cfg.TelegramChatID: map[string]any{
				"workspace":     "agent",
				"allowed_users": []int64{cfg.TelegramUserID},
			},
		},
	}
}

func mailboxConfig(cfg Config) map[string]any {
	return map[string]any{
		"auth": map[string]any{
			"trusted_authserv_id": cfg.AuthservID,
			"tokens": []any{
				map[string]any{
					"name":      "agent",
					"token":     "${MAILBOX_TOKEN}",
					"scopes":    []string{"read", "draft", "organize", "send"},
					"mailboxes": []string{"primary"},
				},
			},
		},
		"state": map[string]string{"path": "/var/lib/mailboxd/state.db"},
		"mailboxes": []any{map[string]any{
			"name": "primary",
			"imap": map[string]any{
				"host":     cfg.IMAPHost,
				"port":     cfg.IMAPPort,
				"tls":      cfg.IMAPTLS,
				"username": "${MAIL_USERNAME}",
				"password": "${MAIL_PASSWORD}",
			},
			"smtp": map[string]any{
				"host":         cfg.SMTPHost,
				"port":         cfg.SMTPPort,
				"tls":          cfg.SMTPTLS,
				"username":     "${MAIL_USERNAME}",
				"password":     "${MAIL_PASSWORD}",
				"from_address": cfg.MailFrom,
				"save_sent":    cfg.MailSaveSent,
			},
		}},
	}
}

func (c Config) validateMailTransport() error {
	for _, port := range []int{c.IMAPPort, c.SMTPPort} {
		if port < 1 || port > 65535 {
			return ctxerrors.Wrap(
				commerr.ErrValidationFailed,
				"mail port outside supported range",
			)
		}
	}

	for _, mode := range []string{c.IMAPTLS, c.SMTPTLS} {
		switch mode {
		case "ssl", "starttls":
		default:
			return ctxerrors.Wrap(
				commerr.ErrValidationFailed,
				"mail TLS must be ssl or starttls",
			)
		}
	}

	return nil
}
