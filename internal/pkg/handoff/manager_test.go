package handoff

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/psyb0t/autonomous-stack/internal/pkg/config"
	"github.com/psyb0t/ctxerrors/commerr"
	"github.com/stretchr/testify/require"
)

type fixtureProcess struct {
	done      chan struct{}
	once      sync.Once
	stopError error
}

func (p *fixtureProcess) URL() string {
	return "https://fixture.trycloudflare.com"
}
func (p *fixtureProcess) Done() <-chan struct{} { return p.done }
func (p *fixtureProcess) Stop(context.Context) error {
	p.once.Do(func() { close(p.done) })

	return p.stopError
}

type fixtureLauncher struct {
	count   int
	process *fixtureProcess
	fail    bool
}

//nolint:ireturn // Implements the tunnel Launcher boundary for tests.
func (l *fixtureLauncher) Start(context.Context, string) (Process, error) {
	l.count++
	if l.fail {
		return nil, commerr.ErrUnavailable
	}

	l.process = &fixtureProcess{done: make(chan struct{})}

	return l.process, nil
}

type fixtureNotifier struct {
	messages []string
	fail     bool
}

func (n *fixtureNotifier) Send(_ context.Context, message string) error {
	if n.fail {
		return commerr.ErrUnavailable
	}

	n.messages = append(n.messages, message)

	return nil
}

func newFixture(t *testing.T) (*Manager, *fixtureLauncher, *fixtureNotifier) {
	t.Helper()

	l, n := &fixtureLauncher{}, &fixtureNotifier{}
	m, err := New(
		config.Config{
			StatePath:      filepath.Join(t.TempDir(), "state.json"),
			RequestTTL:     time.Hour,
			TunnelTTL:      time.Minute,
			StartupTimeout: time.Second,
			ViewerAddress:  "127.0.0.1:8081",
		},
		l,
		n,
	)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, m.Close(t.Context())) })

	return m, l, n
}

func TestManagerLifecycle(t *testing.T) {
	m, launcher, notifications := newFixture(t)
	require.Equal(t, PhaseIdle, m.Status().Phase)
	require.False(t, m.Authorize("anything"))
	state, err := m.Request(
		t.Context(),
		"task",
		"Please complete the challenge",
	)
	require.NoError(t, err)
	require.Equal(t, PhasePending, state.Phase)
	require.Zero(t, launcher.count)
	require.Len(t, notifications.messages, 1)
	duplicate, err := m.Request(t.Context(), "task", "same task")
	require.NoError(t, err)
	require.Equal(t, state.ID, duplicate.ID)
	require.Len(t, notifications.messages, 1)
	_, err = m.Request(t.Context(), "other", "reason")
	require.ErrorIs(t, err, commerr.ErrConflict)
	_, err = m.Start(t.Context(), state.ID, false)
	require.ErrorIs(t, err, commerr.ErrValidationFailed)
	_, err = m.Start(t.Context(), "wrong", true)
	require.ErrorIs(t, err, commerr.ErrValidationFailed)
	require.Zero(t, launcher.count)
	active, err := m.Start(t.Context(), state.ID, true)
	require.NoError(t, err)

	secret := strings.TrimPrefix(active.URL, launcher.process.URL()+"/connect/")
	require.True(t, m.Authorize(secret))
	require.False(t, m.Authorize(secret+"x"))
	require.Len(t, notifications.messages, 2)
	again, err := m.Start(t.Context(), state.ID, true)
	require.NoError(t, err)
	require.Equal(t, active.URL, again.URL)
	require.Equal(t, 1, launcher.count)

	data, err := os.ReadFile(m.cfg.StatePath)
	require.NoError(t, err)
	require.NotContains(t, string(data), secret)
	_, err = m.Finish(t.Context(), "wrong")
	require.ErrorIs(t, err, commerr.ErrNotFound)
	finished, err := m.Finish(t.Context(), state.ID)
	require.NoError(t, err)
	require.Equal(t, PhaseCompleted, finished.Phase)
	require.False(t, m.Authorize(secret))
	_, err = m.Finish(t.Context(), state.ID)
	require.NoError(t, err)
	_, err = m.Start(t.Context(), state.ID, true)
	require.ErrorIs(t, err, commerr.ErrConflict)
	require.NoError(t, m.Tick(t.Context()))
}

func TestManagerFailures(t *testing.T) {
	for _, scenario := range []string{
		"notify-pending",
		"launch",
		"notify-link",
		"expired",
		"process-exit",
		"persist",
		"closed",
		"stop",
	} {
		t.Run(scenario, func(t *testing.T) {
			m, launcher, notifications := newFixture(t)
			if scenario == "notify-pending" {
				notifications.fail = true
			}

			state, err := m.Request(t.Context(), "task", "reason")
			if scenario == "notify-pending" {
				require.Error(t, err)
				require.Equal(t, PhasePending, m.Status().Phase)

				notifications.fail = false
				_, err = m.Request(t.Context(), "task", "reason")
				require.NoError(t, err)

				return
			}

			require.NoError(t, err)

			switch scenario {
			case "launch":
				launcher.fail = true
			case "notify-link":
				notifications.fail = true
			case "expired":
				m.state.ExpiresAt = time.Now().Add(-time.Second)
			case "persist":
				require.NoError(t, os.Remove(m.cfg.StatePath))
				require.NoError(t, os.Mkdir(m.cfg.StatePath, 0o700))
			case "closed":
				require.NoError(t, m.Close(t.Context()))
			}

			result, err := m.Start(t.Context(), state.ID, true)
			if scenario != "process-exit" && scenario != "stop" {
				require.Error(t, err)
				require.False(t, m.Authorize("anything"))

				if scenario == "persist" {
					require.NoError(t, os.Remove(m.cfg.StatePath))
				}

				return
			}

			require.NoError(t, err)

			if scenario == "stop" {
				launcher.process.stopError = commerr.ErrUnavailable
				_, err = m.Finish(t.Context(), state.ID)
				require.Error(t, err)

				launcher.process.stopError = nil
			} else {
				require.NoError(t, launcher.process.Stop(t.Context()))
				require.NoError(t, m.Tick(t.Context()))
				require.Equal(t, PhaseFailed, m.Status().Phase)
			}

			require.False(
				t,
				m.Authorize(
					strings.TrimPrefix(
						result.URL,
						launcher.process.URL()+"/connect/",
					),
				),
			)
		})
	}
}

func TestManagerRestartAndExpiry(t *testing.T) {
	m, launcher, notifications := newFixture(t)
	state, err := m.Request(t.Context(), "task", "reason")
	require.NoError(t, err)
	restarted, err := New(m.cfg, launcher, notifications)
	require.NoError(t, err)
	require.Equal(t, state.ID, restarted.Status().ID)
	_, err = m.Start(t.Context(), state.ID, true)
	require.NoError(t, err)
	restarted, err = New(m.cfg, launcher, notifications)
	require.NoError(t, err)
	require.Equal(t, PhaseExpired, restarted.Status().Phase)

	m.state.ExpiresAt = time.Now().Add(-time.Second)
	require.NoError(t, m.Tick(t.Context()))
	require.Equal(t, PhaseExpired, m.Status().Phase)
	state, err = m.Request(t.Context(), "task", "reason")
	require.NoError(t, err)

	m.state.ExpiresAt = time.Now().Add(-time.Second)
	_, err = m.Request(t.Context(), "task", "reason")
	require.NoError(t, err)
	require.NotEqual(t, state.ID, m.Status().ID)
}

func TestManagerConcurrentRequests(t *testing.T) {
	m, launcher, notifications := newFixture(t)

	var group sync.WaitGroup

	requestErrors := make(chan error, 20)

	for range 20 {
		group.Go(
			func() {
				_, err := m.Request(t.Context(), "task", "reason")
				requestErrors <- err
			},
		)
	}

	group.Wait()
	close(requestErrors)

	for err := range requestErrors {
		require.NoError(t, err)
	}

	require.Len(t, notifications.messages, 1)
	require.Zero(t, launcher.count)

	startErrors := make(chan error, 20)
	id := m.Status().ID

	for range 20 {
		group.Go(func() {
			_, err := m.Start(t.Context(), id, true)
			startErrors <- err
		})
	}

	group.Wait()
	close(startErrors)

	for err := range startErrors {
		require.NoError(t, err)
	}

	require.Equal(t, 1, launcher.count)
	require.Len(t, notifications.messages, 2)

	for _, input := range []struct{ key, reason string }{
		{"", "reason"},
		{"task", ""},
		{strings.Repeat("x", 257), "reason"},
		{"task", strings.Repeat("x", 2049)},
	} {
		_, err := m.Request(t.Context(), input.key, input.reason)
		require.ErrorIs(t, err, commerr.ErrValidationFailed)
	}

	require.NoError(t, m.Close(t.Context()))
	_, err := m.Request(t.Context(), "task", "reason")
	require.Error(t, err)
}
