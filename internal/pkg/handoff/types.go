package handoff

import (
	"context"
	"time"
)

type Phase string

const (
	PhaseIdle      Phase = "idle"
	PhasePending   Phase = "pending"
	PhaseActive    Phase = "active"
	PhaseCompleted Phase = "completed"
	PhaseExpired   Phase = "expired"
	PhaseFailed    Phase = "failed"
)

type State struct {
	ID        string    `json:"id"`
	Key       string    `json:"key"`
	Reason    string    `json:"reason"`
	Phase     Phase     `json:"phase"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
	Notified  bool      `json:"notified"`
}

type Process interface {
	URL() string
	Done() <-chan struct{}
	Stop(context.Context) error
}

type Launcher interface {
	Start(context.Context, string) (Process, error)
}

type Notifier interface {
	Send(context.Context, string) error
}

type StartResult struct {
	State State  `json:"state"`
	URL   string `json:"url"`
}
