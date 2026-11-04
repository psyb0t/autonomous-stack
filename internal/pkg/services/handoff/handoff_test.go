package handoffservice

import (
	"context"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/psyb0t/autonomous-stack/internal/pkg/config"
	"github.com/stretchr/testify/require"
)

func unusedAddress(t *testing.T) string {
	t.Helper()

	listener, err := (&net.ListenConfig{}).Listen(
		t.Context(),
		"tcp",
		"127.0.0.1:0",
	)
	require.NoError(t, err)

	address := listener.Addr().String()
	require.NoError(t, listener.Close())

	return address
}

func TestService(t *testing.T) {
	t.Setenv("AUTONOMOUS_TOKEN", strings.Repeat("x", 32))
	t.Setenv("TELEGRAM_BOT_TOKEN", "EXAMPLE-DO-NOT-USE")
	t.Setenv("TELEGRAM_CHAT_ID", "42")

	service, err := New()
	require.NoError(t, err)
	require.Equal(t, "handoff", service.Name())
	service.cfg.ListenAddress = unusedAddress(t)
	service.cfg.ViewerAddress = unusedAddress(t)
	service.cfg.StatePath = filepath.Join(t.TempDir(), "state.json")
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)

	result := make(chan error, 1)

	go func() { result <- service.Run(ctx) }()

	t.Cleanup(
		func() { require.NoError(t, service.Stop(t.Context())); cancel() },
	)

	client := &http.Client{Timeout: time.Second}
	require.Eventually(t, func() bool {
		req, err := http.NewRequestWithContext(
			ctx,
			http.MethodGet,
			"http://"+service.cfg.ListenAddress+"/health",
			nil,
		)
		if err != nil {
			return false
		}

		response, err := client.Do(req)
		if err != nil {
			return false
		}

		err = response.Body.Close()

		return err == nil && response.StatusCode == http.StatusOK
	}, 5*time.Second, 10*time.Millisecond)

	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		"http://"+service.cfg.ListenAddress+"/mcp",
		nil,
	)
	require.NoError(t, err)
	response, err := client.Do(request)
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, response.StatusCode)
	require.NoError(t, response.Body.Close())
	require.NoError(t, service.Stop(t.Context()))

	select {
	case err := <-result:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("service did not stop")
	}

	t.Setenv("TELEGRAM_CHAT_ID", "invalid")

	_, err = New()
	require.Error(t, err)
}

func TestListenerFailure(t *testing.T) {
	listener, err := (&net.ListenConfig{}).Listen(
		t.Context(),
		"tcp",
		"127.0.0.1:0",
	)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, listener.Close()) })
	service := &Service{cfg: config.Config{
		ListenAddress: listener.Addr().
			String(),
		ViewerAddress:    unusedAddress(t),
		BrowserViewerURL: "http://browser:5900",
		StatePath:        filepath.Join(t.TempDir(), "state.json"),
	}}

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	require.Error(t, service.Run(ctx))
	require.NoError(t, service.Stop(t.Context()))
}
