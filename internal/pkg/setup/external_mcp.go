package setup

import (
	"regexp"

	"github.com/psyb0t/autonomous-stack/internal/pkg/config"
	"github.com/psyb0t/ctxerrors"
	"github.com/psyb0t/ctxerrors/commerr"
)

const (
	minimumModemBoxTokenLength   = 16
	minimumKeePassBoxTokenLength = 32
	maximumExternalTokenLength   = 256
)

var (
	externalTokenPattern = regexp.MustCompile(`^[!-~]+$`)
	modemIDPattern       = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)
)

type externalMCPService struct {
	name        string
	url         string
	token       string
	minTokenLen int
}

func (c Config) externalMCPServices() []externalMCPService {
	return []externalMCPService{
		{
			name:        "modembox",
			url:         c.ModemBoxURL,
			token:       c.ModemBoxToken,
			minTokenLen: minimumModemBoxTokenLength,
		},
		{
			name:        "keepassbox",
			url:         c.KeePassBoxURL,
			token:       c.KeePassBoxToken,
			minTokenLen: minimumKeePassBoxTokenLength,
		},
	}
}

func (service externalMCPService) validate() error {
	if service.url == "" && service.token == "" {
		return nil
	}

	if service.url == "" || service.token == "" {
		return ctxerrors.Wrap(
			commerr.ErrValidationFailed,
			service.name+" MCP URL and token must be set together",
		)
	}

	if err := config.ValidateURL(service.url); err != nil {
		return ctxerrors.Wrap(err, "invalid "+service.name+" MCP URL")
	}

	if len(service.token) < service.minTokenLen ||
		len(service.token) > maximumExternalTokenLength ||
		!externalTokenPattern.MatchString(service.token) {
		return ctxerrors.Wrap(
			commerr.ErrValidationFailed,
			"invalid "+service.name+" bearer token",
		)
	}

	return nil
}

func (c Config) validateExternalMCP() error {
	for _, service := range c.externalMCPServices() {
		if err := service.validate(); err != nil {
			return err
		}
	}

	if c.ModemBoxModemID != "" &&
		(c.ModemBoxURL == "" || c.PhoneNumber == "" ||
			!modemIDPattern.MatchString(c.ModemBoxModemID)) {
		return ctxerrors.Wrap(
			commerr.ErrValidationFailed,
			"MODEMBOX_MODEM_ID needs a valid ID, endpoint and phone number",
		)
	}

	return nil
}
