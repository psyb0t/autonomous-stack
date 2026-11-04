package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/psyb0t/ctxerrors"
)

const (
	argumentCount    = 2
	usageExitCode    = 2
	maximumBodyBytes = 65536
	privateFileMode  = 0o600
)

func main() {
	if len(os.Args) == 3 && os.Args[1] == "--certificates" {
		if err := certificates(os.Args[2]); err != nil {
			slog.Error("fixture certificate failed", "err", err)
			os.Exit(1)
		}

		return
	}

	if len(os.Args) != argumentCount {
		os.Exit(usageExitCode)
	}

	srv := &http.Server{
		Addr: ":8089", ReadHeaderTimeout: time.Second,
		Handler: http.HandlerFunc(handle),
	}
	if err := srv.ListenAndServe(); err != nil {
		slog.Error(
			"fixture failed",
			"err",
			ctxerrors.Wrap(err, "serve fixture"),
		)
		os.Exit(1)
	}
}

func handle(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var message struct {
			Text string `json:"text"`
		}

		body := http.MaxBytesReader(w, r.Body, maximumBodyBytes)

		err := json.NewDecoder(body).Decode(&message)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)

			return
		}

		first, _, _ := strings.Cut(message.Text, "\n")
		if link, ok := strings.CutPrefix(first, "Browser: "); ok {
			if err := os.WriteFile(
				os.Args[1],
				[]byte(link),
				privateFileMode,
			); err != nil {
				w.WriteHeader(http.StatusInternalServerError)

				return
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).
		Encode(map[string]bool{"ok": true}); err != nil {
		slog.Error(
			"fixture response failed",
			"err",
			ctxerrors.Wrap(err, "encode response"),
		)
	}
}
