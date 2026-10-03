// keepsave-runner runs on a separately enrolled rootless Linux host. It has no
// database or vault configuration, and never accepts arbitrary connector code.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/santapong/KeepSave/backend/internal/runner"
)

func main() {
	image := flag.String("image", "", "operator-approved fully qualified image@sha256 digest")
	root := flag.String("attempt-root", "", "operator-private 0700 attempt directory")
	profile := flag.String("seccomp-profile", "", "reviewed Unix-only seccomp profile")
	origin := flag.String("control-origin", "", "control host HTTPS origin")
	cert := flag.String("client-cert", "", "enrolled client certificate file")
	key := flag.String("client-key", "", "private client key file (0600)")
	ca := flag.String("control-ca", "", "control host CA file")
	execute := flag.Bool("execute", false, "explicitly enable claiming and running admitted work")
	once := flag.Bool("once", false, "exit after one claim")
	flag.Parse()
	if flag.NArg() != 0 {
		os.Exit(2)
	}
	p, e := runner.NewPodman(runner.Config{ImageDigest: *image, AttemptRoot: *root, SeccompProfile: *profile})
	if e != nil {
		emit(runner.Diagnostic{Reason: "configuration_invalid"})
		os.Exit(1)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	preflightCtx, preflightCancel := context.WithTimeout(ctx, 5*time.Second)
	d := p.Preflight(preflightCtx)
	preflightCancel()
	emit(d)
	if !d.Available {
		os.Exit(1)
	}
	if !*execute {
		return
	}
	c, e := runner.NewControlClient(runner.TLSConfig{Origin: *origin, CertificateFile: *cert, KeyFile: *key, CAFile: *ca})
	if e != nil {
		emit(runner.Diagnostic{Reason: "mtls_configuration_invalid"})
		os.Exit(1)
	}
	s, e := runner.NewSupervisor(p, c)
	if e != nil {
		os.Exit(1)
	}
	for {
		receipt, e := s.RunOnce(ctx)
		if e == nil {
			emit(receipt)
		} else if !errors.Is(e, runner.ErrNoWork) {
			emit(runner.Diagnostic{Reason: "attempt_stopped"})
			os.Exit(1)
		}
		if *once || ctx.Err() != nil {
			return
		}
		timer := time.NewTimer(time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

func emit(v any) { _ = json.NewEncoder(os.Stdout).Encode(v) }
