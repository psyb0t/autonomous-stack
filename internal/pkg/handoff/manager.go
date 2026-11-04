package handoff

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
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
	mu         sync.Mutex
	cfg        config.Config
	launcher   Launcher
	notifier   Notifier
	state      State
	process    Process
	accessHash [sha256.Size]byte
	link       string
	closed     bool
}

func New(
	cfg config.Config,
	launcher Launcher,
	notifier Notifier,
) (*Manager, error) {
	state, err := loadState(cfg.StatePath)
	if err != nil {
		return nil, ctxerrors.Wrap(err, "load handoff manager")
	}

	if state.Phase == PhaseActive {
		state.Phase = PhaseExpired
		if err := saveState(cfg.StatePath, state); err != nil {
			return nil, ctxerrors.Wrap(err, "invalidate tunnel after restart")
		}
	}

	return &Manager{
		cfg:      cfg,
		launcher: launcher,
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
		"I need help: %s\nRequest: %s\nReply ready %s when available.",
		m.state.Reason, m.state.ID, m.state.ID,
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

	if !userReady || id == "" || id != m.state.ID || m.closed {
		return StartResult{}, ctxerrors.Wrap(
			commerr.ErrValidationFailed,
			"explicit owner readiness for this request is required",
		)
	}

	if !time.Now().Before(m.state.ExpiresAt) {
		return StartResult{}, ctxerrors.Wrap(
			commerr.ErrConflict,
			"handoff request has expired",
		)
	}

	if m.state.Phase == PhaseActive {
		return StartResult{State: m.state, URL: m.link}, nil
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
	secret, err := newSecret()
	if err != nil {
		return StartResult{}, ctxerrors.Wrap(err, "create viewer credential")
	}

	startupCtx, cancel := context.WithTimeout(ctx, m.cfg.StartupTimeout)
	defer cancel()

	process, err := m.launcher.Start(startupCtx, "http://"+m.cfg.ViewerAddress)
	if err != nil {
		return StartResult{}, ctxerrors.Wrap(err, "open browser tunnel")
	}

	m.process = process
	m.accessHash = sha256.Sum256([]byte(secret))
	m.link = process.URL() + "/connect/" + secret
	m.state.Phase = PhaseActive

	m.state.ExpiresAt = time.Now().UTC().Add(m.cfg.TunnelTTL)
	if err := saveState(m.cfg.StatePath, m.state); err != nil {
		stopErr := m.stopLocked(context.WithoutCancel(ctx), PhaseFailed)

		return StartResult{}, ctxerrors.Wrap(
			errors.Join(err, stopErr),
			"persist tunnel state",
		)
	}

	message := fmt.Sprintf(
		"Browser handoff: %s\nExpires at %s. Reply done %s when finished.",
		m.link,
		m.state.ExpiresAt.Format(time.RFC3339),
		m.state.ID,
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

	return StartResult{State: m.state, URL: m.link}, nil
}

func (m *Manager) Finish(ctx context.Context, id string) (State, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

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

func (m *Manager) Authorize(secret string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed || m.state.Phase != PhaseActive ||
		!time.Now().Before(m.state.ExpiresAt) ||
		secret == "" {
		return false
	}

	provided := sha256.Sum256([]byte(secret))

	return subtle.ConstantTimeCompare(provided[:], m.accessHash[:]) == 1
}

func (m *Manager) Tick(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.state.Phase != PhasePending && m.state.Phase != PhaseActive {
		return nil
	}

	if !time.Now().Before(m.state.ExpiresAt) {
		return m.stopLocked(ctx, PhaseExpired)
	}

	if m.process != nil {
		select {
		case <-m.process.Done():
			ctxscope.GetLogger(ctx).
				Warn("browser tunnel exited", "request_id", m.state.ID)

			return m.stopLocked(ctx, PhaseFailed)
		default:
		}
	}

	return nil
}

func (m *Manager) Close(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.closed = true
	if m.process == nil {
		return nil
	}

	return m.stopLocked(ctx, PhaseExpired)
}

func (m *Manager) stopLocked(ctx context.Context, phase Phase) error {
	m.accessHash = [sha256.Size]byte{}

	m.link = ""
	if m.process != nil {
		if err := m.process.Stop(ctx); err != nil {
			return ctxerrors.Wrap(err, "stop owned cloudflared process")
		}

		m.process = nil
	}

	m.state.Phase = phase
	if err := saveState(m.cfg.StatePath, m.state); err != nil {
		return ctxerrors.Wrap(err, "save closed handoff")
	}

	return nil
}
