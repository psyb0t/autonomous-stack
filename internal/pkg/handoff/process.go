package handoff

import (
	"bufio"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"regexp"
	"sync"
	"syscall"
	"time"

	"github.com/psyb0t/ctxerrors"
	"github.com/psyb0t/ctxerrors/commerr"
	"github.com/psyb0t/ctxscope"
)

const stopTimeout = 5 * time.Second

type Cloudflared struct{ Binary string }

type tunnelProcess struct {
	cmd      *exec.Cmd
	done     chan struct{}
	url      string
	stopOnce sync.Once
	stopErr  error
}

//nolint:ireturn // Implements the Launcher process boundary.
func (c Cloudflared) Start(
	ctx context.Context,
	target string,
) (Process, error) {
	// Only startup is bound to the request context.
	//nolint:gosec,noctx // Operator binary; process must outlive the request.
	cmd := exec.Command(
		c.Binary,
		"tunnel",
		"--no-autoupdate",
		"--url",
		target,
		"--protocol",
		"http2",
	)
	cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL}

	pipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, ctxerrors.Wrap(err, "capture tunnel startup")
	}

	if err := cmd.Start(); err != nil {
		return nil, ctxerrors.Wrap(err, "launch cloudflared")
	}

	p := &tunnelProcess{cmd: cmd, done: make(chan struct{})}
	urls := make(chan string, 1)

	go p.readOutput(ctx, pipe, urls)

	return p.waitReady(ctx, urls)
}

func (p *tunnelProcess) readOutput(
	ctx context.Context, pipe io.Reader, urls chan<- string,
) {
	pattern := regexp.MustCompile(
		`https://[a-z0-9]+(?:-[a-z0-9]+)*\.trycloudflare\.com\b`,
	)

	scanner := bufio.NewScanner(pipe)
	for scanner.Scan() {
		if endpoint := pattern.FindString(scanner.Text()); endpoint != "" {
			select {
			case urls <- endpoint:
			default:
			}
		}
	}

	if err := scanner.Err(); err != nil {
		ctxscope.GetLogger(ctx).
			Warn("tunnel output ended", "reason", "read_failed")
	}

	if err := p.cmd.Wait(); err != nil {
		ctxscope.GetLogger(ctx).
			Debug("cloudflared exited", "reason", "nonzero_exit")
	}

	close(p.done)
}

func (p *tunnelProcess) waitReady(
	ctx context.Context, urls <-chan string,
) (*tunnelProcess, error) {
	select {
	case p.url = <-urls:
		select {
		case <-p.done:
			return nil, ctxerrors.Wrap(
				commerr.ErrUnavailable,
				"tunnel exited during startup",
			)
		default:
			return p, nil
		}
	case <-ctx.Done():
		return nil, ctxerrors.Wrap(
			errors.Join(ctx.Err(), p.Stop(context.WithoutCancel(ctx))),
			"tunnel startup deadline",
		)
	case <-p.done:
		return nil, ctxerrors.Wrap(
			commerr.ErrUnavailable,
			"cloudflared exited before a tunnel was ready",
		)
	}
}

func (p *tunnelProcess) URL() string           { return p.url }
func (p *tunnelProcess) Done() <-chan struct{} { return p.done }

func (p *tunnelProcess) Stop(_ context.Context) error {
	p.stopOnce.Do(func() {
		select {
		case <-p.done:
			return
		default:
		}

		if err := p.cmd.Process.Signal(
			os.Interrupt,
		); err != nil &&
			!errors.Is(err, os.ErrProcessDone) {
			p.stopErr = ctxerrors.Wrap(err, "interrupt cloudflared")
		}

		timer := time.NewTimer(stopTimeout)
		defer timer.Stop()

		select {
		case <-p.done:
		case <-timer.C:
			if err := p.cmd.Process.Kill(); err != nil &&
				!errors.Is(err, os.ErrProcessDone) {
				p.stopErr = ctxerrors.Wrap(
					errors.Join(p.stopErr, err),
					"kill cloudflared",
				)
			}

			<-p.done
		}
	})

	return p.stopErr
}
