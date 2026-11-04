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

func newFixture(t *testing.T) (*Manager, *fixtureNotifier) {
	t.Helper()

	n := &fixtureNotifier{}
	m, err := New(
		config.Config{
			StatePath:  filepath.Join(t.TempDir(), "state.json"),
			RequestTTL: time.Hour,
			ViewerURL:  "http://autonomous-stack",
		},
		n,
	)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, m.Close(t.Context())) })

	return m, n
}

func TestManagerLifecycle(t *testing.T) {
	m, notifications := newFixture(t)
	require.Equal(t, PhaseIdle, m.Status().Phase)
	state, err := m.Request(
		t.Context(),
		"task",
		"Please complete the challenge",
	)
	require.NoError(t, err)
	require.Equal(t, PhasePending, state.Phase)
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
	active, err := m.Start(t.Context(), state.ID, true)
	require.NoError(t, err)

	require.Equal(t, "http://autonomous-stack", active.URL)
	require.Len(t, notifications.messages, 2)
	again, err := m.Start(t.Context(), state.ID, true)
	require.NoError(t, err)
	require.Equal(t, active.URL, again.URL)

	data, err := os.ReadFile(m.cfg.StatePath)
	require.NoError(t, err)
	require.NotContains(t, string(data), "/connect/")
	_, err = m.Finish(t.Context(), "wrong")
	require.ErrorIs(t, err, commerr.ErrNotFound)
	finished, err := m.Finish(t.Context(), state.ID)
	require.NoError(t, err)
	require.Equal(t, PhaseCompleted, finished.Phase)
	_, err = m.Finish(t.Context(), state.ID)
	require.NoError(t, err)
	_, err = m.Start(t.Context(), state.ID, true)
	require.ErrorIs(t, err, commerr.ErrConflict)
	require.NoError(t, m.Tick(t.Context()))
}

func TestManagerFailures(t *testing.T) {
	for _, scenario := range []string{
		"notify-pending",
		"notify-link",
		"expired",
		"persist",
		"closed",
	} {
		t.Run(scenario, func(t *testing.T) {
			m, notifications := newFixture(t)
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

			_, err = m.Start(t.Context(), state.ID, true)
			require.Error(t, err)

			if scenario == "persist" {
				require.NoError(t, os.Remove(m.cfg.StatePath))
			}
		})
	}
}

func TestManagerRestartAndExpiry(t *testing.T) {
	m, notifications := newFixture(t)
	state, err := m.Request(t.Context(), "task", "reason")
	require.NoError(t, err)
	restarted, err := New(m.cfg, notifications)
	require.NoError(t, err)
	require.Equal(t, state.ID, restarted.Status().ID)
	_, err = m.Start(t.Context(), state.ID, true)
	require.NoError(t, err)
	restarted, err = New(m.cfg, notifications)
	require.NoError(t, err)
	require.Equal(t, PhaseActive, restarted.Status().Phase)

	m.state.ExpiresAt = time.Now().Add(-time.Second)
	require.NoError(t, m.Tick(t.Context()))
	require.Equal(t, PhaseActive, m.Status().Phase)
	_, err = m.Finish(t.Context(), "")
	require.NoError(t, err)
	state, err = m.Request(t.Context(), "task", "reason")
	require.NoError(t, err)

	m.state.ExpiresAt = time.Now().Add(-time.Second)
	_, err = m.Request(t.Context(), "task", "reason")
	require.NoError(t, err)
	require.NotEqual(t, state.ID, m.Status().ID)
}

func TestManagerConcurrentRequests(t *testing.T) {
	m, notifications := newFixture(t)

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

func TestActiveCoordinationHasNoDeadline(t *testing.T) {
	m, notifications := newFixture(t)
	state, err := m.Request(t.Context(), "task", "reason")
	require.NoError(t, err)
	active, err := m.Start(t.Context(), "", true)
	require.NoError(t, err)
	require.True(t, active.State.ExpiresAt.IsZero())
	require.NotContains(t, notifications.messages[0], state.ID)
	require.NotContains(t, notifications.messages[1], state.ID)
	require.Contains(t, notifications.messages[0], "Reply ready")
	require.Contains(t, notifications.messages[1], "Reply done")
	require.NotContains(t, notifications.messages[1], "/connect/")

	m.state.ExpiresAt = time.Now().Add(-time.Hour)
	require.NoError(t, m.Tick(t.Context()))
	require.Equal(t, PhaseActive, m.Status().Phase)
	require.NoError(t, m.Close(t.Context()))
	restarted, err := New(m.cfg, notifications)
	require.NoError(t, err)
	require.Equal(t, PhaseActive, restarted.Status().Phase)
	require.True(t, restarted.Status().ExpiresAt.IsZero())
	_, err = restarted.Finish(t.Context(), "")
	require.NoError(t, err)
	require.Equal(t, PhaseCompleted, restarted.Status().Phase)
}
