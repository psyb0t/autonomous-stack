package handoffservice

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/psyb0t/aichteeteapee"
	"github.com/psyb0t/aichteeteapee/serbewr"
	"github.com/psyb0t/autonomous-stack/internal/pkg/config"
	"github.com/psyb0t/autonomous-stack/internal/pkg/handoff"
	"github.com/psyb0t/autonomous-stack/internal/pkg/http/server"
	"github.com/psyb0t/ctxerrors"
	"github.com/psyb0t/ctxscope"
)

const (
	ServiceName        = "handoff"
	listenerCount      = 2
	headerTimeout      = 5 * time.Second
	maximumHeaderBytes = 16384
	shutdownTimeout    = 10 * time.Second
)

type Service struct {
	mu     sync.Mutex
	cancel context.CancelFunc
	cfg    config.Config
}

func New() (*Service, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, ctxerrors.Wrap(err, "configure handoff service")
	}

	return &Service{cfg: cfg}, nil
}

func (s *Service) Name() string { return ServiceName }

func (s *Service) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	s.mu.Lock()
	s.cancel = cancel
	s.mu.Unlock()

	manager, err := handoff.New(
		s.cfg,
		handoff.Cloudflared{Binary: s.cfg.CloudflaredBinary},
		handoff.NewTelegram(
			s.cfg.TelegramAPIURL,
			s.cfg.TelegramToken,
			s.cfg.TelegramChatID,
		),
	)
	if err != nil {
		return ctxerrors.Wrap(err, "initialize handoff service")
	}

	defer func() {
		if err := manager.Close(context.WithoutCancel(ctx)); err != nil {
			ctxscope.GetLogger(ctx).Error("handoff cleanup failed", "err", err)
		}
	}()

	viewer, err := server.Viewer(manager, s.cfg.BrowserViewerURL)
	if err != nil {
		return ctxerrors.Wrap(err, "initialize viewer proxy")
	}

	apiServer, err := newServer(ctx, s.cfg.ListenAddress)
	if err != nil {
		return ctxerrors.Wrap(err, "initialize MCP listener")
	}

	viewerServer, err := newServer(ctx, s.cfg.ViewerAddress)
	if err != nil {
		return ctxerrors.Wrap(err, "initialize viewer listener")
	}

	apiServer.GetMux().Handle("/mcp", server.MCP(manager, s.cfg.Token))
	apiServer.GetMux().
		HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
			aichteeteapee.WriteJSON(w, http.StatusOK, struct {
				Status string `json:"status"`
			}{Status: "ok"})
		})
	viewerServer.GetMux().Handle("/", viewer)

	results := make(chan error, listenerCount)

	go func() { results <- serve(ctx, apiServer) }()
	go func() { results <- serve(ctx, viewerServer) }()

	ctxscope.GetLogger(ctx).
		Info("handoff service started", "listen_address", s.cfg.ListenAddress)

	return watch(ctx, cancel, manager, results)
}

func watch(
	ctx context.Context, cancel context.CancelFunc,
	manager *handoff.Manager, results <-chan error,
) error {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctxerrors.Wrap(
				errors.Join(<-results, <-results),
				"stop handoff listeners",
			)
		case err := <-results:
			cancel()

			return ctxerrors.Wrap(
				errors.Join(err, <-results),
				"handoff listener exited",
			)
		case <-ticker.C:
			if err := manager.Tick(ctx); err != nil {
				ctxscope.GetLogger(ctx).
					Error("handoff maintenance failed", "err", err)
			}
		}
	}
}

func serve(ctx context.Context, srv *serbewr.Server) error {
	err := srv.Start(ctx, nil)
	if errors.Is(err, context.Canceled) {
		return nil
	}

	return ctxerrors.Wrap(err, "serve handoff endpoint")
}

func newServer(ctx context.Context, address string) (*serbewr.Server, error) {
	srv, err := serbewr.NewWithConfigAndLogger(serbewr.Config{
		ListenAddress:     address,
		ReadHeaderTimeout: headerTimeout,
		ReadTimeout:       time.Minute,
		IdleTimeout:       time.Minute,
		MaxHeaderBytes:    maximumHeaderBytes,
		ShutdownTimeout:   shutdownTimeout,
		ServiceName:       ServiceName,
	}, ctxscope.GetLogger(ctx))
	if err != nil {
		return nil, ctxerrors.Wrap(err, "create HTTP server")
	}

	return srv, nil
}

func (s *Service) Stop(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cancel != nil {
		s.cancel()
	}

	return nil
}
