package main

import (
	"net/http"
	"time"

	"github.com/psyb0t/autonomous-stack/internal/pkg/setup"
	"github.com/psyb0t/ctxerrors"
	"github.com/spf13/cobra"
)

const healthTimeout = 2 * time.Second

func commands() []*cobra.Command {
	return []*cobra.Command{
		{
			Use:   "prepare",
			Short: "Generate private runtime configuration",
			RunE: func(_ *cobra.Command, _ []string) error {
				cfg, err := setup.Load()
				if err != nil {
					return ctxerrors.Wrap(err, "load setup settings")
				}

				if err := setup.Generate(cfg, "/stack"); err != nil {
					return ctxerrors.Wrap(err, "prepare stack")
				}

				return nil
			},
		},
		{
			Use:   "healthcheck",
			Short: "Check the local handoff listener",
			RunE: func(command *cobra.Command, _ []string) error {
				client := &http.Client{Timeout: healthTimeout}

				req, err := http.NewRequestWithContext(
					command.Context(),
					http.MethodGet,
					"http://127.0.0.1:8080/health",
					nil,
				)
				if err != nil {
					return ctxerrors.Wrap(err, "create health request")
				}

				response, err := client.Do(req)
				if err != nil {
					return ctxerrors.Wrap(err, "handoff listener unavailable")
				}

				if err := response.Body.Close(); err != nil {
					return ctxerrors.Wrap(err, "close health response")
				}

				if response.StatusCode != http.StatusOK {
					return ctxerrors.New("handoff listener is not ready")
				}

				return nil
			},
		},
	}
}
