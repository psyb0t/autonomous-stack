package handoff

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/psyb0t/autonomous-stack/internal/pkg/config"
	"github.com/psyb0t/ctxerrors"
	"github.com/psyb0t/ctxerrors/commerr"
)

const privateDirectoryMode = 0o700

func loadState(path string) (State, error) {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return State{Phase: PhaseIdle}, nil
	}

	if err != nil {
		return State{}, ctxerrors.Wrap(err, "open handoff state")
	}

	data, readErr := io.ReadAll(
		io.LimitReader(file, config.MaximumStateBytes+1),
	)

	closeErr := file.Close()
	if err := errors.Join(readErr, closeErr); err != nil {
		return State{}, ctxerrors.Wrap(err, "read handoff state")
	}

	if len(data) > config.MaximumStateBytes {
		return State{}, ctxerrors.Wrap(
			commerr.ErrValidationFailed,
			"handoff state is oversized",
		)
	}

	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return State{}, ctxerrors.Wrap(err, "decode handoff state")
	}

	switch state.Phase {
	case PhaseIdle,
		PhasePending,
		PhaseActive,
		PhaseCompleted,
		PhaseExpired,
		PhaseFailed:
	default:
		return State{}, ctxerrors.Wrap(
			commerr.ErrValidationFailed,
			"unknown persisted handoff phase",
		)
	}

	return state, nil
}

func saveState(path string, state State) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return ctxerrors.Wrap(err, "encode handoff state")
	}

	if err := os.MkdirAll(
		filepath.Dir(path),
		privateDirectoryMode,
	); err != nil {
		return ctxerrors.Wrap(err, "create handoff state directory")
	}

	file, err := os.CreateTemp(filepath.Dir(path), ".handoff-*")
	if err != nil {
		return ctxerrors.Wrap(err, "create atomic handoff state")
	}

	temporary := file.Name()
	_, writeErr := file.Write(data)
	syncErr := file.Sync()

	closeErr := file.Close()
	if err := errors.Join(writeErr, syncErr, closeErr); err != nil {
		removeErr := os.Remove(temporary)

		return ctxerrors.Wrap(
			errors.Join(err, removeErr),
			"write handoff state",
		)
	}

	if err := os.Rename(temporary, path); err != nil {
		removeErr := os.Remove(temporary)

		return ctxerrors.Wrap(
			errors.Join(err, removeErr),
			"replace handoff state",
		)
	}

	return nil
}
