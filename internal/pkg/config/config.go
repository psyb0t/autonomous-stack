package config

import (
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/psyb0t/ctxerrors"
	"github.com/psyb0t/ctxerrors/commerr"
	"github.com/psyb0t/gonfiguration"
)

const (
	MinimumTokenLength  = 32
	MaximumReasonLength = 2048
	MaximumKeyLength    = 256
	MaximumStateBytes   = 65536
)

type Config struct {
	ListenAddress    string        `default:"0.0.0.0:8080"             env:"AUTONOMOUS_LISTEN_ADDRESS"`
	ViewerAddress    string        `default:"0.0.0.0:8081"             env:"AUTONOMOUS_VIEWER_ADDRESS"`
	ViewerURL        string        `default:"http://autonomous-stack"  env:"AUTONOMOUS_VIEWER_URL"`
	BrowserViewerURL string        `default:"http://browser:5900"      env:"AUTONOMOUS_BROWSER_VIEWER_URL"`
	StatePath        string        `default:"/state/handoff.json"      env:"AUTONOMOUS_STATE_PATH"`
	Token            string        `env:"AUTONOMOUS_TOKEN,required"`
	TelegramToken    string        `env:"TELEGRAM_BOT_TOKEN,required"`
	TelegramChatID   int64         `env:"TELEGRAM_CHAT_ID,required"`
	TelegramAPIURL   string        `default:"https://api.telegram.org" env:"AUTONOMOUS_TELEGRAM_API_URL"`
	RequestTTL       time.Duration `default:"24h"                      env:"AUTONOMOUS_REQUEST_TTL"`
}

func Load() (Config, error) {
	var cfg Config
	if err := gonfiguration.Parse(&cfg); err != nil {
		return Config{}, ctxerrors.Wrap(err, "parse handoff configuration")
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, ctxerrors.Wrap(err, "validate handoff configuration")
	}

	return cfg, nil
}

func (c Config) Validate() error {
	if len(strings.TrimSpace(c.Token)) < MinimumTokenLength ||
		c.TelegramToken == "" || c.TelegramChatID == 0 {
		return ctxerrors.Wrap(
			commerr.ErrValidationFailed,
			"configure handoff token and Telegram owner",
		)
	}

	if err := c.validateDeadlines(); err != nil {
		return ctxerrors.Wrap(err, "invalid deadlines")
	}

	if err := c.validateListeners(); err != nil {
		return ctxerrors.Wrap(err, "invalid listeners")
	}

	for _, endpoint := range []string{c.BrowserViewerURL, c.TelegramAPIURL} {
		if err := ValidateURL(endpoint); err != nil {
			return ctxerrors.Wrap(err, "invalid service URL")
		}
	}

	if err := ValidateViewerURL(c.ViewerURL); err != nil {
		return ctxerrors.Wrap(err, "invalid tailnet viewer URL")
	}

	if !strings.HasPrefix(c.StatePath, "/") {
		return ctxerrors.Wrap(
			commerr.ErrValidationFailed,
			"state path must be absolute",
		)
	}

	return nil
}

func (c Config) validateDeadlines() error {
	for _, bound := range []struct{ value, min, max time.Duration }{
		{c.RequestTTL, time.Minute, 7 * 24 * time.Hour},
	} {
		if bound.value < bound.min || bound.value > bound.max {
			return ctxerrors.Wrap(
				commerr.ErrValidationFailed,
				"deadline outside supported bounds",
			)
		}
	}

	return nil
}

func (c Config) validateListeners() error {
	for _, address := range []string{c.ListenAddress, c.ViewerAddress} {
		_, port, err := net.SplitHostPort(address)
		if err != nil {
			return ctxerrors.Wrap(err, "split listener address")
		}

		parsedPort, err := strconv.Atoi(port)
		if err != nil || parsedPort < 1 || parsedPort > 65535 {
			return ctxerrors.Wrap(
				commerr.ErrValidationFailed,
				"invalid listener port",
			)
		}
	}

	return nil
}

func ValidateViewerURL(raw string) error {
	if err := ValidateURL(raw); err != nil {
		return ctxerrors.Wrap(err, "validate viewer origin")
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return ctxerrors.Wrap(
			commerr.ErrValidationFailed,
			"invalid viewer origin",
		)
	}

	if parsed.Path != "" && parsed.Path != "/" {
		return ctxerrors.Wrap(
			commerr.ErrValidationFailed,
			"viewer URL must be an origin without a path",
		)
	}

	return nil
}

func ValidateHostname(name string) error {
	if len(name) > 63 ||
		!regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]*[a-z0-9])?$`).
			MatchString(name) {
		return ctxerrors.Wrap(
			commerr.ErrValidationFailed,
			"use a lowercase DNS label for TS_HOSTNAME",
		)
	}

	return nil
}

func ValidateURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return ctxerrors.Wrap(
			commerr.ErrValidationFailed,
			"invalid service URL syntax",
		)
	}

	if (parsed.Scheme != "http" && parsed.Scheme != "https") ||
		parsed.Hostname() == "" ||
		parsed.User != nil ||
		parsed.RawQuery != "" ||
		parsed.Fragment != "" {
		return ctxerrors.Wrap(
			commerr.ErrValidationFailed,
			"use HTTP(S) without credentials, query or fragment",
		)
	}

	return nil
}
