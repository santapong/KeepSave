package runner

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
)

// Each fixture generates an ephemeral CA and distinct server/client identity.
// No operator key, provider token, database or external provider is involved.
func mtlsFixture(t *testing.T, handler http.Handler) (*ControlClient, *httptest.Server) {
	t.Helper()
	now := time.Now()
	caKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	ca := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "synthetic CA"}, NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	caRaw, e := x509.CreateCertificate(rand.Reader, ca, ca, &caKey.PublicKey, caKey)
	if e != nil {
		t.Fatal(e)
	}
	ca, e = x509.ParseCertificate(caRaw)
	if e != nil {
		t.Fatal(e)
	}
	issue := func(serial int64, usage x509.ExtKeyUsage) ([]byte, []byte) {
		key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		cert := &x509.Certificate{SerialNumber: big.NewInt(serial), Subject: pkix.Name{CommonName: "synthetic identity"}, NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{usage}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}}
		raw, e := x509.CreateCertificate(rand.Reader, cert, ca, &key.PublicKey, caKey)
		if e != nil {
			t.Fatal(e)
		}
		keyRaw, e := x509.MarshalPKCS8PrivateKey(key)
		if e != nil {
			t.Fatal(e)
		}
		return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: raw}), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyRaw})
	}
	serverCert, serverKey := issue(2, x509.ExtKeyUsageServerAuth)
	pair, e := tls.X509KeyPair(serverCert, serverKey)
	if e != nil {
		t.Fatal(e)
	}
	roots := x509.NewCertPool()
	roots.AddCert(ca)
	s := httptest.NewUnstartedServer(handler)
	s.TLS = &tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{pair}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: roots}
	s.StartTLS()
	t.Cleanup(s.Close)
	clientCert, clientKey := issue(3, x509.ExtKeyUsageClientAuth)
	dir := t.TempDir()
	certPath := filepath.Join(dir, "client.pem")
	keyPath := filepath.Join(dir, "client.key")
	caPath := filepath.Join(dir, "ca.pem")
	for path, data := range map[string][]byte{certPath: clientCert, keyPath: clientKey, caPath: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caRaw})} {
		if os.WriteFile(path, data, 0600) != nil {
			t.Fatal("fixture write")
		}
	}
	c, e := NewControlClient(TLSConfig{Origin: s.URL, CertificateFile: certPath, KeyFile: keyPath, CAFile: caPath})
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(c.client.CloseIdleConnections)
	return c, s
}

func TestControlClientMTLSBindsFixedPathsAndTicket(t *testing.T) {
	ticket := testTicket()
	var pathCount atomic.Int32
	fingerprints := make(chan string, 3)
	resultID := uuid.New()
	client, _ := mtlsFixture(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.TLS == nil || len(r.TLS.PeerCertificates) != 1 || r.TLS.Version != tls.VersionTLS13 {
			t.Error("missing verified mutual TLS")
		}
		h := sha256.Sum256(r.TLS.PeerCertificates[0].Raw)
		fingerprints <- hex.EncodeToString(h[:])
		if r.Method != "POST" || r.Header.Get("Content-Type") != "application/json" {
			t.Error("unfixed method")
		}
		pathCount.Add(1)
		switch r.URL.Path {
		case "/api/v1/runner/operations/claim":
			var req ClaimRequest
			if json.NewDecoder(r.Body).Decode(&req) != nil || req.ImageDigest != ticket.ImageDigest {
				t.Error("image not bound")
			}
			_ = json.NewEncoder(w).Encode(ticket)
		case "/api/v1/runner/operations/execute":
			var req ExecuteRequest
			if json.NewDecoder(r.Body).Decode(&req) != nil || req.TicketID != ticket.TicketID || req.Token != ticket.Token || req.Request != ticket.Request() {
				t.Error("ticket not bound")
			}
			_ = json.NewEncoder(w).Encode(ExecuteResponse{Result: json.RawMessage(`{"content":"permitted"}`), Outcome: "succeeded", ReceiptID: resultID})
		case "/api/v1/runner/operations/status":
			var req TicketStatusRequest
			if json.NewDecoder(r.Body).Decode(&req) != nil || req.TicketID != ticket.TicketID || req.Token != ticket.Token {
				t.Error("status identity missing")
			}
			_ = json.NewEncoder(w).Encode(TicketStatusResponse{Active: true})
		default:
			t.Error("arbitrary control path")
		}
	}))
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:1")
	claimed, e := client.Claim(context.Background(), ticket.ImageDigest)
	if e != nil {
		t.Fatal(e)
	}
	if client.CertificateSHA256 != <-fingerprints {
		t.Fatal("enrollment fingerprint mismatch")
	}
	result, e := client.Execute(context.Background(), claimed, claimed.Request())
	if e != nil || result.ReceiptID != resultID {
		t.Fatal("execute failed")
	}
	if active, e := client.Active(context.Background(), claimed); e != nil || !active {
		t.Fatal("status failed")
	}
	if pathCount.Load() != 3 {
		t.Fatal("unexpected extra transport call")
	}
}

func TestControlClientNoWorkSafeErrorsUnknownJSONAndBounds(t *testing.T) {
	for _, mode := range []string{"no_work", "denied", "unknown_field", "oversize", "malformed", "provider_error", "redirect"} {
		t.Run(mode, func(t *testing.T) {
			c, _ := mtlsFixture(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch mode {
				case "no_work":
					w.WriteHeader(204)
				case "denied":
					w.WriteHeader(403)
					_, _ = w.Write([]byte("provider-token-canary"))
				case "unknown_field":
					_, _ = w.Write([]byte(`{"active":true,"provider_token":"canary"}`))
				case "oversize":
					_, _ = w.Write([]byte(strings.Repeat("x", MaxRequestBytes+1)))
				case "malformed":
					_, _ = w.Write([]byte(`{"active":true} {}`))
				case "provider_error":
					w.WriteHeader(503)
					_, _ = w.Write([]byte("provider-token-canary"))
				case "redirect":
					w.Header().Set("Location", "https://127.0.0.1:1/leak")
					w.WriteHeader(307)
				}
			}))
			_, e := c.Active(context.Background(), testTicket())
			if e == nil || strings.Contains(e.Error(), "canary") {
				t.Fatal("unsafe control response accepted")
			}
			if mode == "no_work" && e != ErrNoWork {
				t.Fatal("no work mismatch")
			}
		})
	}
}

func TestControlClientExecuteCannotChangeRequest(t *testing.T) {
	var calls atomic.Int32
	c, _ := mtlsFixture(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); w.WriteHeader(403) }))
	ticket := testTicket()
	request := ticket.Request()
	request.RunID = uuid.New()
	if _, e := c.Execute(context.Background(), ticket, request); e != ErrDenied || calls.Load() != 0 {
		t.Fatal("changed operation reached network")
	}
}

func TestControlClientTLSAndPrivateKeyRequired(t *testing.T) {
	for _, origin := range []string{"http://example.invalid", "https://user:pass@example.invalid", "https://example.invalid/api", "https://example.invalid?x=y", "https://example.invalid#x"} {
		if _, e := NewControlClient(TLSConfig{Origin: origin}); e != ErrInvalid {
			t.Fatal("unsafe origin accepted")
		}
	}
	dir := t.TempDir()
	key := filepath.Join(dir, "key")
	_ = os.WriteFile(key, []byte("not a key"), 0644)
	if _, e := NewControlClient(TLSConfig{Origin: "https://example.invalid", KeyFile: key}); e != ErrUnavailable {
		t.Fatal("public key file accepted")
	}
	_ = os.Chmod(key, 0600)
	link := filepath.Join(dir, "link")
	_ = os.Symlink(key, link)
	if privateFile(link) {
		t.Fatal("key symlink accepted")
	}
}

func TestControlClientDeadlineCancelsSyntheticDispatch(t *testing.T) {
	c, _ := mtlsFixture(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(200 * time.Millisecond):
		}
	}))
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	started := time.Now()
	_, e := c.Active(ctx, testTicket())
	if e == nil || time.Since(started) > time.Second {
		t.Fatal("client deadline not enforced")
	}
}
