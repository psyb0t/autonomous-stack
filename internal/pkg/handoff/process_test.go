package handoff

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCloudflared(t *testing.T) {
	for _, scenario := range []struct {
		name, program string
		wantError     bool
	}{
		{name: "ready", program: `
echo 'https://local-fixture.trycloudflare.com' >&2
trap 'exit 0' INT TERM
while :; do sleep 0.1; done`},
		{name: "exits", program: "exit 1", wantError: true},
		{name: "deadline", program: `trap 'exit 0' INT TERM
while :; do sleep 0.1; done`, wantError: true},
		{name: "force-kill", program: `
echo 'https://local-fixture.trycloudflare.com' >&2
trap '' INT TERM
while :; do sleep 0.1; done`},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "cloudflared")
			//nolint:gosec // Child-process fixture must be executable.
			require.NoError(
				t,
				os.WriteFile(
					path,
					[]byte("#!/bin/sh\n"+scenario.program+"\n"),
					0o700,
				),
			)

			ctx, cancel := context.WithTimeout(t.Context(), time.Second)
			defer cancel()

			process, err := (Cloudflared{Binary: path}).Start(
				ctx,
				"http://127.0.0.1:8081",
			)
			if scenario.wantError {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)
			require.Equal(
				t,
				"https://local-fixture.trycloudflare.com",
				process.URL(),
			)
			cancel()

			select {
			case <-process.Done():
				t.Fatal("request cancellation killed an accepted tunnel")
			default:
			}

			require.NoError(t, process.Stop(t.Context()))
			require.NoError(t, process.Stop(t.Context()))

			select {
			case <-process.Done():
			case <-time.After(time.Second):
				t.Fatal("child not reaped")
			}
		})
	}

	_, err := (Cloudflared{Binary: "/missing/cloudflared"}).Start(
		t.Context(),
		"http://127.0.0.1:8081",
	)
	require.Error(t, err)
}
