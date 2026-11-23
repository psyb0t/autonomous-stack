package setup

import (
	"strings"

	"github.com/psyb0t/autonomous-stack/internal/pkg/config"
	"github.com/psyb0t/ctxerrors"
	"github.com/psyb0t/ctxerrors/commerr"
)

const (
	piBox         = "pibox"
	maximumEffort = "max"
)

type stdioMCPServer struct {
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env"`
}

func (c Config) agentEnabled() bool {
	return c.local("agent") || c.local("claude")
}

func (c Config) validateHarness() error {
	switch c.HarnessPlatform {
	case "", "claudebox":
		return nil
	case piBox:
		if strings.TrimSpace(c.ZAIAuthToken) == "" ||
			strings.TrimSpace(c.ZAIModel) == "" {
			return ctxerrors.Wrap(
				commerr.ErrValidationFailed,
				"PiBox requires a Z.ai token and model",
			)
		}

		if err := config.ValidateURL(c.ZAIBaseURL); err != nil {
			return ctxerrors.Wrap(err, "invalid PiBox Z.ai base URL")
		}

		return nil
	default:
		return ctxerrors.Wrap(
			commerr.ErrValidationFailed,
			"HARNESS_PLATFORM must be claudebox or pibox",
		)
	}
}

func (c Config) runtimeModel() string {
	if c.HarnessPlatform == piBox {
		return c.ZAIModel
	}

	return c.Model
}

func (c Config) runtimeEffort() string {
	if c.HarnessPlatform == piBox && c.Effort == maximumEffort {
		return "xhigh"
	}

	return c.Effort
}

func (c Config) projectMCPServers() any {
	servers := configuredMCPServers(c)
	if c.HarnessPlatform != piBox {
		return servers
	}

	bridges := make(map[string]stdioMCPServer, len(servers))
	for name, server := range servers {
		bridges[name] = stdioMCPServer{
			Command: "node",
			Args:    []string{"/opt/stack/pibox-mcp-bridge.js"},
			Env: map[string]string{
				"AUTONOMOUS_MCP_URL":           server.URL,
				"AUTONOMOUS_MCP_AUTHORIZATION": server.Headers["Authorization"],
			},
		}
	}

	return bridges
}
