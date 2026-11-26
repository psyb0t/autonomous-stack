package setup

import (
	"encoding/json"
	"errors"
	"net/mail"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/psyb0t/autonomous-stack/internal/pkg/config"
	"github.com/psyb0t/ctxerrors"
	"github.com/psyb0t/ctxerrors/commerr"
	"github.com/psyb0t/gonfiguration"
)

const privateDirectoryMode = 0o700

type Config struct {
	Profiles        string `env:"STACK_PROFILES"`
	HarnessPlatform string `env:"HARNESS_PLATFORM"`
	ZAIAuthToken    string `env:"PIBOX_ZAI_AUTH_TOKEN"`
	ZAIModel        string `default:"glm-5.3-flash"        env:"PIBOX_ZAI_DEFAULT_MODEL"`
	TelegramChatID  int64  `env:"TELEGRAM_CHAT_ID"`
	TelegramUserID  int64  `env:"TELEGRAM_USER_ID"`
	TelegramToken   string `env:"TELEGRAM_BOT_TOKEN"`
	Model           string `default:"sonnet"               env:"CLAUDE_MODEL"`
	Effort          string `default:"high"                 env:"CLAUDE_EFFORT"`
	Schedule        string `default:"*/30 * * * *"         env:"HEARTBEAT_SCHEDULE"`
	Heartbeat       string `env:"HEARTBEAT_PROMPT"`
	SystemPrompt    string `env:"AGENT_PROMPT"`
	HandoffURL      string `env:"HANDOFF_MCP_URL"`
	HandoffToken    string `env:"AUTONOMOUS_TOKEN"`
	BrowserURL      string `env:"BROWSER_MCP_URL"`
	BrowserToken    string `env:"BROWSER_TOKEN"`
	MailboxURL      string `env:"MAILBOX_MCP_URL"`
	MailboxToken    string `env:"MAILBOX_TOKEN"`
	IMAPHost        string `env:"MAIL_IMAP_HOST"`
	IMAPPort        int    `default:"993"                  env:"MAIL_IMAP_PORT"`
	IMAPTLS         string `default:"ssl"                  env:"MAIL_IMAP_TLS"`
	SMTPHost        string `env:"MAIL_SMTP_HOST"`
	SMTPPort        int    `default:"587"                  env:"MAIL_SMTP_PORT"`
	SMTPTLS         string `default:"starttls"             env:"MAIL_SMTP_TLS"`
	MailUsername    string `env:"MAIL_USERNAME"`
	MailPassword    string `env:"MAIL_PASSWORD"`
	MailFrom        string `env:"MAIL_FROM"`
	MailSaveSent    bool   `default:"true"                 env:"MAIL_SAVE_SENT"`
	AuthservID      string `env:"MAIL_TRUSTED_AUTHSERV_ID"`
	PhoneURL        string `env:"DIKCIZ_MCP_URL"`
	PhoneToken      string `env:"DIKCIZ_API_KEY"`
	PhoneNumber     string `env:"AUTONOMOUS_PHONE_NUMBER"`
	ModemBoxURL     string `env:"MODEMBOX_MCP_URL"`
	ModemBoxToken   string `env:"MODEMBOX_TOKEN"`
	ModemBoxModemID string `env:"MODEMBOX_MODEM_ID"`
	KeePassBoxURL   string `env:"KEEPASSBOX_MCP_URL"`
	KeePassBoxToken string `env:"KEEPASSBOX_TOKEN"`
	TailnetHostname string `default:"autonomous-stack"     env:"TS_HOSTNAME"`
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

	if c.agentEnabled() {
		if err := c.validateHarness(); err != nil {
			return ctxerrors.Wrap(err, "agent harness")
		}

		if err := c.validateOwner(); err != nil {
			return ctxerrors.Wrap(err, "owner configuration")
		}

		if err := c.validateAgent(); err != nil {
			return ctxerrors.Wrap(err, "agent configuration")
		}
	}

	if err := c.validateMail(); err != nil {
		return ctxerrors.Wrap(err, "mail configuration")
	}

	if err := c.validatePhone(); err != nil {
		return ctxerrors.Wrap(err, "phone configuration")
	}

	if err := c.validateExternalMCP(); err != nil {
		return ctxerrors.Wrap(err, "external MCP configuration")
	}

	return nil
}

func (c Config) local(name string) bool {
	if c.Profiles == "" {
		return name == "claude" || name == "handoff" ||
			name == "browser" || name == "mailbox"
	}

	for profile := range strings.SplitSeq(c.Profiles, ",") {
		if strings.TrimSpace(profile) == name ||
			strings.TrimSpace(profile) == "*" {
			return true
		}
	}

	return false
}

func (c Config) validateOwner() error {
	if c.TelegramChatID == 0 || c.TelegramUserID <= 0 ||
		strings.TrimSpace(c.TelegramToken) == "" {
		return ctxerrors.Wrap(
			commerr.ErrValidationFailed,
			"set the Telegram bot, owner chat and owner user ID",
		)
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
		c.runtimeModel() == "" {
		return ctxerrors.Wrap(
			commerr.ErrValidationFailed,
			"model and prompts cannot be empty",
		)
	}

	switch c.Effort {
	case "low", "medium", "high", maximumEffort:
	default:
		return ctxerrors.Wrap(
			commerr.ErrValidationFailed,
			"unsupported Claude effort",
		)
	}

	return nil
}

func (c Config) validateMail() error {
	if !c.local("mailbox") {
		if c.MailFrom != "" {
			if _, err := mail.ParseAddress(c.MailFrom); err != nil {
				return ctxerrors.Wrap(err, "invalid mail sender")
			}
		}

		return nil
	}

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
	if c.PhoneNumber != "" &&
		!regexp.MustCompile(`^\+[1-9][0-9]{1,14}$`).MatchString(c.PhoneNumber) {
		return ctxerrors.Wrap(
			commerr.ErrValidationFailed,
			"verification phone must use E.164 format",
		)
	}

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

func configuredMCPServers(cfg Config) map[string]mcpServer {
	servers := make(map[string]mcpServer)
	if cfg.PhoneURL != "" {
		servers["phone"] = mcpServer{
			Type: "http",
			URL:  cfg.PhoneURL,
			Headers: map[string]string{
				"Authorization": "Bearer " + cfg.PhoneToken,
			},
		}
	}

	for _, service := range cfg.mcpServices() {
		if endpoint := service.endpoint(); endpoint != "" {
			servers[service.name] = mcpServer{
				Type: "http",
				URL:  endpoint,
				Headers: map[string]string{
					"Authorization": "Bearer " + service.token,
				},
			}
		}
	}

	return servers
}

func Generate(cfg Config, root string) error {
	if err := cfg.Validate(); err != nil {
		return ctxerrors.Wrap(err, "refuse invalid generated configuration")
	}

	for name, value := range cfg.generatedFiles() {
		if err := writeJSON(filepath.Join(root, name), value); err != nil {
			return ctxerrors.Wrap(err, "write private runtime configuration")
		}
	}

	if cfg.agentEnabled() {
		if err := prepareIdentity(cfg, root); err != nil {
			return ctxerrors.Wrap(err, "prepare account password storage")
		}
	}

	return nil
}

func (c Config) generatedFiles() map[string]any {
	files := make(map[string]any)
	if c.agentEnabled() {
		files["config/generated/cron.yml"] = cronConfig(c)
		files["config/generated/telegram.yml"] = telegramConfig(c)
		files["data/workspace/agent/.mcp.json"] = map[string]any{
			"mcpServers": c.projectMCPServers(),
		}
		settingsPath := "data/workspace/agent/.claude/settings.local.json"
		files[settingsPath] = map[string]any{
			"enableAllProjectMcpServers": true,
		}
	}

	if c.local("mailbox") {
		path := "config/mailbox/config.yaml"
		if c.Profiles == "" {
			path = "config/generated/mailbox.yml"
		}

		files[path] = mailboxConfig(c)
	}

	if c.local("keepassbox") {
		files["data/keepassbox/access.local.json"] = map[string]any{
			"clients": []map[string]string{{
				"owner": "automation",
				"token": c.KeePassBoxToken,
			}},
		}
	}

	return files
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
	schedule := cfg.Schedule
	if cfg.HarnessPlatform == piBox && len(strings.Fields(schedule)) == 5 {
		schedule = "0 " + schedule
	}

	return map[string]any{
		"model":                cfg.runtimeModel(),
		"effort":               cfg.runtimeEffort(),
		"telegram_chat_id":     cfg.TelegramChatID,
		"append_system_prompt": cfg.SystemPrompt,
		"jobs": []any{
			map[string]any{
				"name":        "heartbeat",
				"schedule":    schedule,
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
			"model":                cfg.runtimeModel(),
			"effort":               cfg.runtimeEffort(),
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
