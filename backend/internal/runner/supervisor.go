package runner

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type Supervisor struct {
	podman       *Podman
	control      Control
	pollInterval time.Duration
}

func NewSupervisor(p *Podman, c Control) (*Supervisor, error) {
	if p == nil || c == nil {
		return nil, ErrInvalid
	}
	return &Supervisor{p, c, time.Second}, nil
}

// RunOnce refuses to claim any work until all required local capabilities are
// available. A failed attempt is never retried by this supervisor.
func (s *Supervisor) RunOnce(ctx context.Context) (Receipt, error) {
	preflightCtx, cancelPreflight := context.WithTimeout(ctx, 5*time.Second)
	d := s.podman.Preflight(preflightCtx)
	cancelPreflight()
	if !d.Available {
		return Receipt{Outcome: d.Reason}, ErrUnavailable
	}
	ticket, e := s.control.Claim(ctx, s.podman.config.ImageDigest)
	if e != nil {
		return Receipt{}, e
	}
	return s.execute(ctx, ticket)
}

func (s *Supervisor) execute(ctx context.Context, ticket Ticket) (receipt Receipt, err error) {
	if ticket.Validate(s.podman.config.ImageDigest, time.Now()) != nil {
		return Receipt{}, ErrDenied
	}
	deadline := time.Now().Add(MaxAttemptDuration)
	if ticket.ExpiresAt.Before(deadline) {
		deadline = ticket.ExpiresAt
	}
	attemptCtx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()
	dir, e := os.MkdirTemp(s.podman.config.AttemptRoot, "attempt-")
	if e != nil {
		return Receipt{}, ErrUnavailable
	}
	defer os.RemoveAll(dir)
	if os.Chmod(dir, 0750) != nil {
		return Receipt{}, ErrUnavailable
	}
	request, e := json.Marshal(ticket.Request())
	if e != nil || len(request) > MaxRequestBytes {
		return Receipt{}, ErrInvalid
	}
	if os.WriteFile(filepath.Join(dir, "request.json"), request, 0440) != nil {
		return Receipt{}, ErrUnavailable
	}
	relay, e := startRelay(attemptCtx, dir, ticket, s.control, cancel)
	if e != nil {
		return Receipt{}, e
	}
	defer relay.close()
	args, e := s.podman.arguments(ticket, dir)
	if e != nil {
		return Receipt{}, e
	}
	defer func() {
		cancel()
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		// --ignore handles the normal --rm teardown. --force kills any process
		// left after cancellation, expiry, denial or supervisor command failure.
		if _, e := s.podman.command(cleanupCtx, []string{"rm", "--force", "--ignore", "--time=0", "--", containerName(ticket)}, 0); e != nil {
			err = ErrUnavailable
		}
	}()
	watchDone := make(chan struct{})
	go func() {
		defer close(watchDone)
		ticker := time.NewTicker(s.pollInterval)
		defer ticker.Stop()
		for {
			select {
			case <-attemptCtx.Done():
				return
			case <-ticker.C:
				checkCtx, checkCancel := context.WithTimeout(attemptCtx, 2*time.Second)
				active, e := s.control.Active(checkCtx, ticket)
				checkCancel()
				if e != nil || !active {
					cancel()
					return
				}
			}
		}
	}()
	_, e = s.podman.command(attemptCtx, args, 0)
	cancel()
	<-watchDone
	receipt = relay.result()
	if e != nil || receipt.Outcome != "succeeded" {
		return receipt, ErrUnavailable
	}
	return receipt, nil
}
