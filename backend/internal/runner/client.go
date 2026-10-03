package runner

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"
)

// Control is supervisor-only. A connector cannot construct a Control client.
type Control interface {
	Claim(context.Context, string) (Ticket, error)
	Execute(context.Context, Ticket, OperationRequest) (ExecuteResponse, error)
	Active(context.Context, Ticket) (bool, error)
}

type TLSConfig struct{ Origin, CertificateFile, KeyFile, CAFile string }
type ControlClient struct {
	origin            string
	client            *http.Client
	CertificateSHA256 string
}

func NewControlClient(c TLSConfig) (*ControlClient, error) {
	u, e := url.Parse(c.Origin)
	if e != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return nil, ErrInvalid
	}
	// Never inherit proxy settings or take client key material from environment.
	if !privateFile(c.KeyFile) {
		return nil, ErrUnavailable
	}
	cert, e := tls.LoadX509KeyPair(c.CertificateFile, c.KeyFile)
	if e != nil || len(cert.Certificate) == 0 {
		return nil, ErrUnavailable
	}
	leaf, e := x509.ParseCertificate(cert.Certificate[0])
	now := time.Now()
	if e != nil || now.Before(leaf.NotBefore) || !now.Before(leaf.NotAfter) {
		return nil, ErrUnavailable
	}
	clientUsage := false
	for _, usage := range leaf.ExtKeyUsage {
		if usage == x509.ExtKeyUsageClientAuth {
			clientUsage = true
		}
	}
	if !clientUsage {
		return nil, ErrUnavailable
	}
	ca, e := os.ReadFile(c.CAFile)
	if e != nil {
		return nil, ErrUnavailable
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(ca) {
		return nil, ErrUnavailable
	}
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS13, RootCAs: roots, Certificates: []tls.Certificate{cert}, ServerName: u.Hostname()}
	transport := &http.Transport{TLSClientConfig: tlsConfig, Proxy: nil, MaxConnsPerHost: 2, ResponseHeaderTimeout: 5 * time.Second, MaxResponseHeaderBytes: 8 * 1024}
	h := sha256.Sum256(cert.Certificate[0])
	return &ControlClient{origin: c.Origin, CertificateSHA256: hex.EncodeToString(h[:]), client: &http.Client{Transport: transport, Timeout: MaxAttemptDuration, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}

func privateFile(path string) bool {
	info, e := os.Lstat(path)
	return e == nil && info.Mode().IsRegular() && info.Mode().Perm()&0077 == 0
}

func strictJSON(data []byte, out any) error {
	if uniqueJSON(json.NewDecoder(bytes.NewReader(data)), 0) != nil {
		return ErrInvalid
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if d.Decode(out) != nil {
		return ErrInvalid
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return ErrInvalid
	}
	return nil
}

// Reject duplicate keys before typed decoding, so one body has one meaning.
func uniqueJSON(d *json.Decoder, depth int) error {
	if depth > 64 {
		return ErrInvalid
	}
	token, e := d.Token()
	if e != nil {
		return ErrInvalid
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		keys := make(map[string]bool)
		for d.More() {
			token, e := d.Token()
			if e != nil {
				return ErrInvalid
			}
			key, ok := token.(string)
			if !ok || keys[key] {
				return ErrInvalid
			}
			keys[key] = true
			if uniqueJSON(d, depth+1) != nil {
				return ErrInvalid
			}
		}
		end, e := d.Token()
		if e != nil || end != json.Delim('}') {
			return ErrInvalid
		}
	case '[':
		for d.More() {
			if uniqueJSON(d, depth+1) != nil {
				return ErrInvalid
			}
		}
		end, e := d.Token()
		if e != nil || end != json.Delim(']') {
			return ErrInvalid
		}
	default:
		return ErrInvalid
	}
	return nil
}

func (c *ControlClient) post(ctx context.Context, path string, payload, out any, limit int64) error {
	body, e := json.Marshal(payload)
	if e != nil {
		return ErrInvalid
	}
	req, e := http.NewRequestWithContext(ctx, http.MethodPost, c.origin+path, bytes.NewReader(body))
	if e != nil {
		return ErrUnavailable
	}
	req.Header.Set("Content-Type", "application/json")
	resp, e := c.client.Do(req)
	if e != nil {
		return ErrUnavailable
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNoContent {
		return ErrNoWork
	}
	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusConflict || resp.StatusCode == http.StatusGone {
		return ErrDenied
	}
	if resp.StatusCode != http.StatusOK {
		return ErrUnavailable
	}
	data, e := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if e != nil || int64(len(data)) > limit {
		return ErrUnavailable
	}
	if strictJSON(data, out) != nil {
		return ErrUnavailable
	}
	return nil
}

func (c *ControlClient) Claim(ctx context.Context, image string) (Ticket, error) {
	if !ValidImage(image) {
		return Ticket{}, ErrInvalid
	}
	var ticket Ticket
	if e := c.post(ctx, "/api/v1/runner/operations/claim", ClaimRequest{image}, &ticket, MaxRequestBytes); e != nil {
		return Ticket{}, e
	}
	if ticket.Validate(image, time.Now()) != nil {
		return Ticket{}, ErrDenied
	}
	return ticket, nil
}

func (c *ControlClient) Execute(ctx context.Context, ticket Ticket, request OperationRequest) (ExecuteResponse, error) {
	if ticket.Validate(ticket.ImageDigest, time.Now()) != nil || ticket.Request() != request {
		return ExecuteResponse{}, ErrDenied
	}
	var result ExecuteResponse
	if e := c.post(ctx, "/api/v1/runner/operations/execute", ExecuteRequest{ticket.TicketID, ticket.Token, request}, &result, MaxResultBytes+MaxRequestBytes); e != nil {
		return ExecuteResponse{}, e
	}
	if len(result.Result) > MaxResultBytes || !json.Valid(result.Result) || result.Outcome != "succeeded" || result.ReceiptID == [16]byte{} {
		return ExecuteResponse{}, ErrUnavailable
	}
	return result, nil
}

func (c *ControlClient) Active(ctx context.Context, ticket Ticket) (bool, error) {
	var out TicketStatusResponse
	e := c.post(ctx, "/api/v1/runner/operations/status", TicketStatusRequest{ticket.TicketID, ticket.Token}, &out, MaxRequestBytes)
	return out.Active, e
}
