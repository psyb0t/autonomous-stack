package handoff

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/psyb0t/autonomous-stack/internal/pkg/config"
	"github.com/psyb0t/ctxerrors"
	"github.com/psyb0t/ctxerrors/commerr"
	"github.com/psyb0t/ctxscope"
)

const secretBytes = 32

type Manager struct {
	mu       sync.Mutex
	cfg      config.Config
	notifier Notifier
	state    State
	closed   bool
}

func New(
	cfg config.Config,
	notifier Notifier,
) (*Manager, error) {
	state, err := loadState(cfg.StatePath)
	if err != nil {
		return nil, ctxerrors.Wrap(err, "load handoff manager")
	}

	if state.Phase == PhaseActive {
		state.ExpiresAt = time.Time{}
		if err := saveState(cfg.StatePath, state); err != nil {
			return nil, ctxerrors.Wrap(
				err,
				"preserve active coordination after restart",
			)
		}
	}

	return &Manager{
		cfg:      cfg,
		notifier: notifier,
		state:    state,
	}, nil
}

func newSecret() (string, error) {
	buffer := make([]byte, secretBytes)
	if _, err := rand.Read(buffer); err != nil {
		return "", ctxerrors.Wrap(err, "generate handoff secret")
	}

	return hex.EncodeToString(buffer), nil
}

func (m *Manager) Request(
	ctx context.Context,
	key, reason string,
) (State, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key, reason = strings.TrimSpace(key), strings.TrimSpace(reason)
	if !validRequest(key, reason) {
		return State{}, ctxerrors.Wrap(
			commerr.ErrValidationFailed,
			"supply a bounded task key and reason",
		)
	}

	if m.closed {
		return State{}, ctxerrors.Wrap(
			commerr.ErrConflict,
			"handoff service is stopping",
		)
	}

	if m.state.Phase == PhasePending && !time.Now().Before(m.state.ExpiresAt) {
		m.state.Phase = PhaseExpired
	}

	if m.state.Phase == PhasePending || m.state.Phase == PhaseActive {
		if m.state.Key != key {
			return State{}, ctxerrors.Wrap(
				commerr.ErrConflict,
				"finish the existing handoff first",
			)
		}
	} else {
		if err := m.createRequest(key, reason); err != nil {
			return State{}, ctxerrors.Wrap(err, "create pending handoff")
		}
	}

	if err := m.notifyPending(ctx); err != nil {
		return m.state, ctxerrors.Wrap(err, "notify pending request")
	}

	ctxscope.GetLogger(ctx).
		Info("human help requested", "request_id", m.state.ID)

	return m.state, nil
}

func validRequest(key, reason string) bool {
	return key != "" && len(key) <= config.MaximumKeyLength &&
		reason != "" && len(reason) <= config.MaximumReasonLength
}

func (m *Manager) notifyPending(ctx context.Context) error {
	if m.state.Notified {
		return nil
	}

	message := fmt.Sprintf(
		"I need help: %s\nBrowser: %s\nReply ready when available.",
		m.state.Reason, m.cfg.ViewerURL,
	)
	if err := m.notifier.Send(ctx, message); err != nil {
		return ctxerrors.Wrap(err, "notify owner; pending request retained")
	}

	m.state.Notified = true
	if err := saveState(m.cfg.StatePath, m.state); err != nil {
		return ctxerrors.Wrap(err, "record notification")
	}

	return nil
}

func (m *Manager) createRequest(key, reason string) error {
	id, err := newSecret()
	if err != nil {
		return ctxerrors.Wrap(err, "create request ID")
	}

	state := State{
		ID:        id,
		Key:       key,
		Reason:    reason,
		Phase:     PhasePending,
		CreatedAt: time.Now().UTC(),
		ExpiresAt: time.Now().UTC().Add(m.cfg.RequestTTL),
	}
	if err := saveState(m.cfg.StatePath, state); err != nil {
		return ctxerrors.Wrap(err, "persist pending handoff")
	}

	m.state = state

	return nil
}

func (m *Manager) Start(
	ctx context.Context,
	id string,
	userReady bool,
) (StartResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if id == "" {
		id = m.state.ID
	}

	if !userReady || id == "" || id != m.state.ID || m.closed {
		return StartResult{}, ctxerrors.Wrap(
			commerr.ErrValidationFailed,
			"explicit owner readiness for this request is required",
		)
	}

	if m.state.Phase == PhasePending && !time.Now().Before(m.state.ExpiresAt) {
		return StartResult{}, ctxerrors.Wrap(
			commerr.ErrConflict,
			"handoff request has expired",
		)
	}

	if m.state.Phase == PhaseActive {
		return StartResult{State: m.state, URL: m.cfg.ViewerURL}, nil
	}

	if m.state.Phase != PhasePending {
		return StartResult{}, ctxerrors.Wrap(
			commerr.ErrConflict,
			"handoff is not pending",
		)
	}

	return m.openLocked(ctx)
}

func (m *Manager) openLocked(ctx context.Context) (StartResult, error) {
	m.state.Phase = PhaseActive

	m.state.ExpiresAt = time.Time{}
	if err := saveState(m.cfg.StatePath, m.state); err != nil {
		stopErr := m.stopLocked(context.WithoutCancel(ctx), PhaseFailed)

		return StartResult{}, ctxerrors.Wrap(
			errors.Join(err, stopErr),
			"persist active coordination",
		)
	}

	message := fmt.Sprintf(
		`Browser: %s
Reply done when finished so I can resume. Browser access stays available.`,
		m.cfg.ViewerURL,
	)
	if err := m.notifier.Send(ctx, message); err != nil {
		stopErr := m.stopLocked(context.WithoutCancel(ctx), PhaseFailed)

		return StartResult{}, ctxerrors.Wrap(
			errors.Join(err, stopErr),
			"deliver browser link",
		)
	}

	ctxscope.GetLogger(ctx).
		Info("browser handoff opened", "request_id", m.state.ID)

	return StartResult{State: m.state, URL: m.cfg.ViewerURL}, nil
}

func (m *Manager) Finish(ctx context.Context, id string) (State, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if id == "" {
		id = m.state.ID
	}

	if id == "" || id != m.state.ID {
		return State{}, ctxerrors.Wrap(
			commerr.ErrNotFound,
			"unknown handoff request",
		)
	}

	if m.state.Phase == PhaseCompleted {
		return m.state, nil
	}

	if m.state.Phase != PhaseActive && m.state.Phase != PhasePending {
		return State{}, ctxerrors.Wrap(
			commerr.ErrConflict,
			"handoff is already closed",
		)
	}

	if err := m.stopLocked(ctx, PhaseCompleted); err != nil {
		return m.state, ctxerrors.Wrap(err, "finish handoff")
	}

	ctxscope.GetLogger(ctx).Info("browser handoff closed", "request_id", id)

	return m.state, nil
}

func (m *Manager) Status() State {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.state
}

func (m *Manager) Tick(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.state.Phase != PhasePending {
		return nil
	}

	if !time.Now().Before(m.state.ExpiresAt) {
		return m.stopLocked(ctx, PhaseExpired)
	}

	return nil
}

func (m *Manager) Close(_ context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.closed = true

	return nil
}

func (m *Manager) stopLocked(_ context.Context, phase Phase) error {
	m.state.Phase = phase
	if err := saveState(m.cfg.StatePath, m.state); err != nil {
		return ctxerrors.Wrap(err, "save closed handoff")
	}

	return nil
}
