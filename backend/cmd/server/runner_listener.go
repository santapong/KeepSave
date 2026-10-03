package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"github.com/santapong/KeepSave/backend/internal/api"
	"github.com/santapong/KeepSave/backend/internal/config"
	"net/http"
	"os"
	"time"
)

// The public reverse proxy must never terminate runner authentication. Verified
// peer certificates reach the restricted handler directly on this listener.
func runnerListener(p config.Platform, h *api.ToolPlatformHandler) (*http.Server, error) {
	if p.RunnerAddress == "" {
		return nil, nil
	}
	ca, e := os.ReadFile(p.RunnerClientCA)
	if e != nil {
		return nil, fmt.Errorf("runner trust configuration unavailable")
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(ca) {
		return nil, fmt.Errorf("runner trust configuration invalid")
	}
	cert, e := tls.LoadX509KeyPair(p.RunnerCert, p.RunnerKey)
	if e != nil {
		return nil, fmt.Errorf("runner server identity unavailable")
	}
	return &http.Server{Addr: p.RunnerAddress, Handler: api.RunnerRouter(h), TLSConfig: &tls.Config{MinVersion: tls.VersionTLS13, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: roots, Certificates: []tls.Certificate{cert}}, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 40 * time.Second, IdleTimeout: 30 * time.Second}, nil
}
func closeRunner(ctx context.Context, s *http.Server) {
	if s != nil {
		_ = s.Shutdown(ctx)
	}
}
