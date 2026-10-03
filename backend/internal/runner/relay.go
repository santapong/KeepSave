package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Receipt contains only control-host-authored metadata. Connector stdout is not
// an operation result and is never forwarded, interpreted or persisted.
type Receipt struct {
	OperationID uuid.UUID `json:"operation_id"`
	ReceiptID   uuid.UUID `json:"receipt_id,omitempty"`
	Outcome     string    `json:"outcome"`
}

type relay struct {
	ctx      context.Context
	ticket   Ticket
	control  Control
	cancel   context.CancelFunc
	mu       sync.Mutex
	used     bool
	receipt  Receipt
	server   *http.Server
	listener net.Listener
}

func startRelay(ctx context.Context, dir string, ticket Ticket, control Control, cancel context.CancelFunc) (*relay, error) {
	socket := filepath.Join(dir, "execute.sock")
	l, e := net.Listen("unix", socket)
	if e != nil {
		return nil, ErrUnavailable
	}
	if os.Chmod(socket, 0660) != nil {
		l.Close()
		return nil, ErrUnavailable
	}
	r := &relay{ctx: ctx, ticket: ticket, control: control, cancel: cancel, listener: l, receipt: Receipt{OperationID: ticket.OperationID, Outcome: "not_dispatched"}}
	r.server = &http.Server{Handler: r, ReadHeaderTimeout: time.Second, ReadTimeout: 2 * time.Second, WriteTimeout: MaxAttemptDuration, MaxHeaderBytes: 4096, BaseContext: func(net.Listener) context.Context { return ctx }}
	go func() { _ = r.server.Serve(l) }()
	return r, nil
}

func (r *relay) close() {
	_ = r.server.Close()
	r.mu.Lock()
	r.ticket.Token = ""
	r.mu.Unlock()
}
func (r *relay) result() Receipt { r.mu.Lock(); defer r.mu.Unlock(); return r.receipt }

func safeRelayError(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(struct {
		Code string `json:"code"`
	}{code})
}

func (r *relay) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost || req.URL.Path != "/execute" || req.URL.RawQuery != "" || req.Header.Get("Content-Type") != "application/json" {
		safeRelayError(w, 403, "operation_denied")
		r.cancel()
		return
	}
	body, e := io.ReadAll(io.LimitReader(req.Body, MaxRequestBytes+1))
	if e != nil || len(body) > MaxRequestBytes {
		safeRelayError(w, 413, "request_invalid")
		r.cancel()
		return
	}
	var request OperationRequest
	if strictJSON(body, &request) != nil || request.Validate() != nil {
		safeRelayError(w, 403, "operation_denied")
		r.cancel()
		return
	}
	r.mu.Lock()
	if r.used {
		r.mu.Unlock()
		safeRelayError(w, 409, "attempt_consumed")
		r.cancel()
		return
	}
	if r.ctx.Err() != nil || !r.ticket.ExpiresAt.After(time.Now()) || request != r.ticket.Request() {
		r.mu.Unlock()
		safeRelayError(w, 403, "operation_denied")
		r.cancel()
		return
	}
	r.used = true
	ticket := r.ticket
	r.receipt.Outcome = "uncertain"
	r.mu.Unlock()
	response, e := r.control.Execute(r.ctx, ticket, request)
	if e != nil || response.Outcome != "succeeded" || response.ReceiptID == uuid.Nil || len(response.Result) > MaxResultBytes || !json.Valid(response.Result) {
		safeRelayError(w, 403, "operation_denied")
		r.cancel()
		return
	}
	r.mu.Lock()
	r.receipt = Receipt{ticket.OperationID, response.ReceiptID, response.Outcome}
	r.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

// ExecuteConnector is the reviewed connector protocol, limited to a request
// file and one Unix socket. It prints metadata and discards permitted content.
func ExecuteConnector(ctx context.Context, requestFile, socket string, out io.Writer) error {
	info, e := os.Lstat(requestFile)
	if e != nil || !info.Mode().IsRegular() || info.Size() > MaxRequestBytes {
		return ErrInvalid
	}
	f, e := os.Open(requestFile)
	if e != nil {
		return ErrInvalid
	}
	defer f.Close()
	body, e := io.ReadAll(io.LimitReader(f, MaxRequestBytes+1))
	if e != nil || len(body) > MaxRequestBytes {
		return ErrInvalid
	}
	var request OperationRequest
	if strictJSON(body, &request) != nil || request.Validate() != nil {
		return ErrInvalid
	}
	info, e = os.Lstat(socket)
	if e != nil || info.Mode()&os.ModeSocket == 0 {
		return ErrUnavailable
	}
	d := &net.Dialer{Timeout: time.Second}
	transport := &http.Transport{Proxy: nil, DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) { return d.DialContext(ctx, "unix", socket) }, MaxConnsPerHost: 1, DisableKeepAlives: true, ResponseHeaderTimeout: MaxAttemptDuration, MaxResponseHeaderBytes: 4096}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: MaxAttemptDuration, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	req, e := http.NewRequestWithContext(ctx, http.MethodPost, "http://relay/execute", bytes.NewReader(body))
	if e != nil {
		return ErrInvalid
	}
	req.Header.Set("Content-Type", "application/json")
	response, e := client.Do(req)
	if e != nil {
		return ErrUnavailable
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return ErrDenied
	}
	data, e := io.ReadAll(io.LimitReader(response.Body, MaxResultBytes+MaxRequestBytes+1))
	if e != nil || len(data) > MaxResultBytes+MaxRequestBytes {
		return ErrUnavailable
	}
	var result ExecuteResponse
	if strictJSON(data, &result) != nil || result.Outcome != "succeeded" || result.ReceiptID == uuid.Nil || !json.Valid(result.Result) || len(result.Result) > MaxResultBytes {
		return ErrUnavailable
	}
	return json.NewEncoder(out).Encode(Receipt{request.OperationID, result.ReceiptID, result.Outcome})
}
