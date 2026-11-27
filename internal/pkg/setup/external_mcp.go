package setup

import (
	"regexp"
	"strings"

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
	localURL    string
	token       string
	minTokenLen int
}

func (c Config) mcpServices() []externalMCPService {
	services := []externalMCPService{
		{
			"browser", c.BrowserURL, "http://browser:8080/mcp/",
			c.BrowserToken, config.MinimumTokenLength,
		},
		{
			"mailbox", c.MailboxURL, "http://mailbox:8000/mcp",
			c.MailboxToken, config.MinimumTokenLength,
		},
		{
			"handoff", c.HandoffURL, "http://handoff:8080/mcp",
			c.HandoffToken, config.MinimumTokenLength,
		},
		{
			name:        "modembox",
			url:         c.ModemBoxURL,
			localURL:    "http://modembox:8000/mcp",
			token:       c.ModemBoxToken,
			minTokenLen: minimumModemBoxTokenLength,
		},
		{
			name:        "keepassbox",
			url:         c.KeePassBoxURL,
			localURL:    "http://keepassbox:8000/mcp",
			token:       c.KeePassBoxToken,
			minTokenLen: minimumKeePassBoxTokenLength,
		},
	}
	for index := range services {
		if !c.local(services[index].name) {
			services[index].localURL = ""
		}
	}

	return services
}

func (service externalMCPService) endpoint() string {
	if service.localURL != "" {
		return service.localURL
	}

	return service.url
}

func (service externalMCPService) validate() error {
	if service.localURL != "" && service.url != "" {
		return ctxerrors.Wrap(
			commerr.ErrValidationFailed,
			"disable the "+service.name+" profile to use a remote MCP URL",
		)
	}

	endpoint := service.endpoint()
	if endpoint == "" {
		return nil
	}

	if err := config.ValidateURL(endpoint); err != nil {
		return ctxerrors.Wrap(err, "invalid "+service.name+" MCP URL")
	}

	if len(service.token) < service.minTokenLen ||
		len(service.token) > maximumExternalTokenLength ||
		strings.HasPrefix(service.token, "REPLACE_ME") ||
		!externalTokenPattern.MatchString(service.token) {
		return ctxerrors.Wrap(
			commerr.ErrValidationFailed,
			"invalid "+service.name+" bearer token",
		)
	}

	return nil
}

func (c Config) validateExternalMCP() error {
	for _, service := range c.mcpServices() {
		if err := service.validate(); err != nil {
			return err
		}
	}

	if c.ModemBoxModemID != "" && (c.Profiles == "" || c.modemConfigured()) &&
		(!c.modemConfigured() || c.PhoneNumber == "" ||
			!modemIDPattern.MatchString(c.ModemBoxModemID)) {
		return ctxerrors.Wrap(
			commerr.ErrValidationFailed,
			"MODEMBOX_MODEM_ID needs a valid ID, endpoint and phone number",
		)
	}

	return nil
}

func (c Config) modemConfigured() bool {
	return c.ModemBoxURL != "" || c.local("modembox")
}
