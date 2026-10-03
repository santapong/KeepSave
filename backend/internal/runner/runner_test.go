package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
)

func testTicket() Ticket {
	t := Ticket{TicketID: uuid.New(), OperationID: uuid.New(), RunID: uuid.New(), GrantID: uuid.New(), Attempt: 1, Fence: 1, ImageDigest: "registry.example/keepsave/connector@sha256:" + strings.Repeat("a", 64), Nonce: strings.Repeat("b", 64), ExpiresAt: time.Now().Add(30 * time.Second), Kind: "read_file", Arguments: Arguments{Path: "README.md"}, Token: strings.Repeat("C", 43)}
	t.RequestDigest = RequestDigest(t.Kind, t.Arguments)
	return t
}

type fixtureControl struct {
	ticket       Ticket
	executeCalls atomic.Int32
	claimCalls   atomic.Int32
	response     ExecuteResponse
	executeError error
	active       bool
	block        bool
}

func (f *fixtureControl) Claim(context.Context, string) (Ticket, error) {
	f.claimCalls.Add(1)
	return f.ticket, nil
}
func (f *fixtureControl) Execute(ctx context.Context, t Ticket, r OperationRequest) (ExecuteResponse, error) {
	f.executeCalls.Add(1)
	if t != f.ticket || r != t.Request() {
		return ExecuteResponse{}, ErrDenied
	}
	if f.block {
		<-ctx.Done()
		return ExecuteResponse{}, ErrUnavailable
	}
	return f.response, f.executeError
}
func (f *fixtureControl) Active(context.Context, Ticket) (bool, error) { return f.active, nil }
func successfulControl(ticket Ticket) *fixtureControl {
	return &fixtureControl{ticket: ticket, active: true, response: ExecuteResponse{Result: json.RawMessage(`{"content":"permitted-content-canary"}`), Outcome: "succeeded", ReceiptID: uuid.New()}}
}

func TestTicketBindingsAndTypedArguments(t *testing.T) {
	ticket := testTicket()
	if ticket.Validate(ticket.ImageDigest, time.Now()) != nil {
		t.Fatal("valid ticket denied")
	}
	cases := map[string]func(*Ticket){"image": func(t *Ticket) { t.ImageDigest = "image:latest" }, "expired": func(t *Ticket) { t.ExpiresAt = time.Now().Add(-time.Second) }, "token": func(t *Ticket) { t.Token = "credential" }, "nonce": func(t *Ticket) { t.Nonce = "short" }, "fence": func(t *Ticket) { t.Fence++ }, "digest": func(t *Ticket) { t.RequestDigest = strings.Repeat("d", 64) }, "kind": func(t *Ticket) { t.Kind = "arbitrary_shell" }, "operation": func(t *Ticket) { t.OperationID = uuid.Nil }, "path": func(t *Ticket) { t.Arguments.Path = "../token"; t.RequestDigest = RequestDigest(t.Kind, t.Arguments) }}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			bad := ticket
			change(&bad)
			if bad.Validate(ticket.ImageDigest, time.Now()) == nil {
				t.Fatal("malformed authority accepted")
			}
		})
	}
	for _, path := range []string{"", "/etc/passwd", "a//b", "a/../b", "a\\b", "x\x00y", "a\tb", "a\x01b", string([]byte{0xff}), strings.Repeat("x", 513)} {
		r := ticket.Request()
		r.Arguments.Path = path
		r.RequestDigest = RequestDigest(r.Kind, r.Arguments)
		if r.Validate() == nil {
			t.Fatalf("accepted unsafe path %q", path)
		}
	}
}

func TestRequestFileContainsNoTicketToken(t *testing.T) {
	ticket := testTicket()
	b, e := json.Marshal(ticket.Request())
	if e != nil {
		t.Fatal(e)
	}
	for _, secret := range []string{ticket.Token, `"token"`, `"certificate"`, `"database_url"`, `"provider_token"`, `"client_key"`} {
		if bytes.Contains(b, []byte(secret)) {
			t.Fatal("credential field crossed boundary")
		}
	}
}

func requestRelay(r *relay, request OperationRequest, extra string) *httptest.ResponseRecorder {
	b, _ := json.Marshal(request)
	if extra != "" {
		b = append(b[:len(b)-1], []byte(extra)...)
	}
	req := httptest.NewRequest("POST", "http://relay/execute", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestRelayRejectsEveryAlteredBindingAndUnknownField(t *testing.T) {
	base := testTicket()
	cases := map[string]func(*OperationRequest){"operation": func(r *OperationRequest) { r.OperationID = uuid.New() }, "run": func(r *OperationRequest) { r.RunID = uuid.New() }, "grant": func(r *OperationRequest) { r.GrantID = uuid.New() }, "attempt": func(r *OperationRequest) { r.Attempt++; r.Fence++ }, "image": func(r *OperationRequest) { r.ImageDigest = strings.Replace(r.ImageDigest, "aaaa", "bbbb", 1) }, "nonce": func(r *OperationRequest) { r.Nonce = strings.Repeat("e", 64) }, "arguments": func(r *OperationRequest) {
		r.Arguments.Path = "another.md"
		r.RequestDigest = RequestDigest(r.Kind, r.Arguments)
	}}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			f := successfulControl(base)
			r := &relay{ctx: ctx, ticket: base, control: f, cancel: cancel}
			req := base.Request()
			change(&req)
			w := requestRelay(r, req, "")
			if w.Code != 403 || f.executeCalls.Load() != 0 || ctx.Err() == nil {
				t.Fatal("changed authority reached control")
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f := successfulControl(base)
	r := &relay{ctx: ctx, ticket: base, control: f, cancel: cancel}
	w := requestRelay(r, base.Request(), `,"url":"https://attacker.invalid"}`)
	if w.Code != 403 || f.executeCalls.Load() != 0 {
		t.Fatal("unknown relay field accepted")
	}
}

func TestRelayConsumesExactlyOnceIncludingConcurrentRequests(t *testing.T) {
	ticket := testTicket()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f := successfulControl(ticket)
	r := &relay{ctx: ctx, ticket: ticket, control: f, cancel: cancel}
	var wg sync.WaitGroup
	codes := make(chan int, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); codes <- requestRelay(r, ticket.Request(), "").Code }()
	}
	wg.Wait()
	close(codes)
	if f.executeCalls.Load() != 1 {
		t.Fatal("duplicate provider dispatch")
	}
	seen := map[int]bool{}
	for code := range codes {
		seen[code] = true
	}
	if !seen[200] || !seen[409] {
		t.Fatal("single-use response mismatch")
	}
	if r.result().Outcome != "succeeded" {
		t.Fatal("safe receipt missing")
	}
}

func TestRelayExpiryAndDeniedProviderCancel(t *testing.T) {
	for _, mode := range []string{"expiry", "broker_deny"} {
		t.Run(mode, func(t *testing.T) {
			ticket := testTicket()
			f := successfulControl(ticket)
			if mode == "expiry" {
				ticket.ExpiresAt = time.Now().Add(-time.Second)
				f.ticket = ticket
			} else {
				f.executeError = errors.New("synthetic-provider-token-canary")
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			r := &relay{ctx: ctx, ticket: ticket, control: f, cancel: cancel}
			w := requestRelay(r, ticket.Request(), "")
			if w.Code != 403 || ctx.Err() == nil || strings.Contains(w.Body.String(), "canary") || strings.Contains(w.Body.String(), ticket.Token) {
				t.Fatal("denial did not safely stop attempt")
			}
			if mode == "expiry" && f.executeCalls.Load() != 0 {
				t.Fatal("expired dispatch")
			}
		})
	}
}

func TestRelayBoundsBodyMethodAndRoute(t *testing.T) {
	for _, tc := range []struct{ method, path, body string }{{"GET", "/execute", "{}"}, {"POST", "/other", "{}"}, {"POST", "/execute?upstream=x", "{}"}, {"POST", "/execute", strings.Repeat("x", MaxRequestBytes+1)}, {"POST", "/execute", "{} {}"}} {
		ctx, cancel := context.WithCancel(context.Background())
		f := successfulControl(testTicket())
		r := &relay{ctx: ctx, ticket: f.ticket, control: f, cancel: cancel}
		req := httptest.NewRequest(tc.method, "http://relay"+tc.path, strings.NewReader(tc.body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		cancel()
		if w.Code < 400 || f.executeCalls.Load() != 0 {
			t.Fatal("unbounded relay surface")
		}
	}
}

func TestStrictJSONRejectsDuplicateFieldsAndExcessDepth(t *testing.T) {
	for _, body := range []string{`{"active":true,"active":false}`, `{"active":true,"unknown":1}`, `{"active":true} {}`, strings.Repeat("[", 65) + "0" + strings.Repeat("]", 65)} {
		var result TicketStatusResponse
		if strictJSON([]byte(body), &result) == nil {
			t.Fatal("ambiguous or unbounded JSON accepted")
		}
	}
	if strictJSON([]byte(`{"active":true}`), new(TicketStatusResponse)) != nil {
		t.Fatal("valid typed JSON denied")
	}
}

func TestConnectorUnixRelayPrintsOnlyMetadata(t *testing.T) {
	ticket := testTicket()
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f := successfulControl(ticket)
	r, e := startRelay(ctx, dir, ticket, f, cancel)
	if e != nil {
		t.Fatal(e)
	}
	defer r.close()
	b, _ := json.Marshal(ticket.Request())
	requestFile := filepath.Join(dir, "request.json")
	if os.WriteFile(requestFile, b, 0440) != nil {
		t.Fatal("request write")
	}
	var out bytes.Buffer
	if ExecuteConnector(ctx, requestFile, filepath.Join(dir, "execute.sock"), &out) != nil {
		t.Fatal("connector failed")
	}
	if !strings.Contains(out.String(), ticket.OperationID.String()) || strings.Contains(out.String(), "permitted-content-canary") || strings.Contains(out.String(), ticket.Token) {
		t.Fatal("connector output crossed metadata boundary")
	}
	info, _ := os.Lstat(filepath.Join(dir, "execute.sock"))
	if info.Mode().Perm() != 0660 {
		t.Fatal("socket permission too broad")
	}
	if ExecuteConnector(ctx, requestFile, filepath.Join(dir, "execute.sock"), io.Discard) == nil {
		t.Fatal("repeated connector use accepted")
	}
}

func TestConnectorRejectsRequestSymlinkAndOversize(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "request")
	_ = os.WriteFile(file, []byte(strings.Repeat("x", MaxRequestBytes+1)), 0600)
	if ExecuteConnector(context.Background(), file, "unused", io.Discard) == nil {
		t.Fatal("oversize file accepted")
	}
	link := filepath.Join(dir, "link")
	_ = os.Symlink(file, link)
	if ExecuteConnector(context.Background(), link, "unused", io.Discard) == nil {
		t.Fatal("symlink request accepted")
	}
}

func podmanFixture(t *testing.T) *Podman {
	t.Helper()
	// Unix socket names have a kernel length limit; keep this fixture short.
	root, e := os.MkdirTemp("", "kr-")
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	_ = os.Chmod(root, 0700)
	profile := filepath.Join(root, "seccomp.json")
	src := []byte(`{"defaultAction":"SCMP_ACT_ERRNO","syscalls":[{"names":["socket"],"action":"SCMP_ACT_ALLOW","args":[{"index":0,"value":1,"op":"SCMP_CMP_EQ"}]}]}`)
	_ = os.WriteFile(profile, src, 0600)
	p, e := NewPodman(Config{ImageDigest: testTicket().ImageDigest, AttemptRoot: root, SeccompProfile: profile})
	if e != nil {
		t.Fatal(e)
	}
	return p
}

func TestPodmanCommandHasOnlyBoundedStaticControls(t *testing.T) {
	p := podmanFixture(t)
	ticket := testTicket()
	dir, e := os.MkdirTemp(p.config.AttemptRoot, "attempt-")
	if e != nil {
		t.Fatal(e)
	}
	args, e := p.arguments(ticket, dir)
	if e != nil {
		t.Fatal(e)
	}
	joined := strings.Join(args, "\n")
	for _, flag := range []string{"--pull=never", "--network=none", "--read-only", "--read-only-tmpfs=false", "--tmpfs=/scratch:rw,noexec,nosuid,nodev,size=64m", "--cpus=1", "--memory=256m", "--memory-swap=256m", "--pids-limit=32", "--cap-drop=ALL", "--security-opt=no-new-privileges", "--security-opt=seccomp=", "--user=65532:65532", "--log-driver=none", "--unsetenv-all", "--http-proxy=false"} {
		if !strings.Contains(joined, flag) {
			t.Fatalf("missing control %s", flag)
		}
	}
	volumes := 0
	for _, arg := range args {
		if strings.HasPrefix(arg, "--volume=") {
			volumes++
			if arg != "--volume="+dir+":/relay:ro,nosuid,nodev,noexec" {
				t.Fatal("unexpected mount")
			}
		}
		for _, bad := range []string{ticket.Token, "--privileged", "--env-host", "docker.sock", "podman.sock", "--network=host", "DATABASE_URL", "MASTER_KEY"} {
			if strings.Contains(arg, bad) {
				t.Fatal("credential or escape channel")
			}
		}
	}
	if volumes != 1 || args[len(args)-3] != ticket.ImageDigest {
		t.Fatal("not one digest-bound image/mount")
	}
	if _, e = p.arguments(ticket, p.config.AttemptRoot); e == nil {
		t.Fatal("root mounted")
	}
	ticket.ImageDigest = "image:latest"
	if _, e = p.arguments(ticket, dir); e == nil {
		t.Fatal("tag image allowed")
	}
}

func TestPreflightMissingCPURefusesBeforeImageOrClaim(t *testing.T) {
	p := podmanFixture(t)
	calls := 0
	p.command = func(context.Context, []string, int) ([]byte, error) {
		calls++
		return []byte(`{"host":{"os":"linux","cgroupVersion":"v2","cgroupControllers":["memory","pids"],"security":{"rootless":true,"seccompEnabled":true}}}`), nil
	}
	d := p.Preflight(context.Background())
	if d.Available || d.IsolationAccepted {
		t.Fatal("synthetic fixture claimed isolation")
	}
	if os.Getuid() != 0 && (d.Reason != "cgroup_cpu_missing" || calls != 1) {
		t.Fatal("missing CPU not denied")
	}
	f := successfulControl(testTicket())
	s, _ := NewSupervisor(p, f)
	_, _ = s.RunOnce(context.Background())
	if f.claimCalls.Load() != 0 {
		t.Fatal("claimed without supported controls")
	}
}

func TestPrivateConfigurationRefusesBroadPermissionsOrIPSeccomp(t *testing.T) {
	p := podmanFixture(t)
	_ = os.Chmod(p.config.AttemptRoot, 0755)
	if _, e := NewPodman(p.config); e == nil {
		t.Fatal("broad root permissions")
	}
	_ = os.Chmod(p.config.AttemptRoot, 0700)
	for _, rule := range []string{
		`{"names":["socket"],"action":"SCMP_ACT_ALLOW"}`,
		`{"names":["socket"],"action":"SCMP_ACT_LOG"}`,
		`{"names":["socket"],"action":"SCMP_ACT_NOTIFY"}`,
		`{"names":["clone"],"action":"SCMP_ACT_ALLOW"}`,
		`{"names":["clone3"],"action":"SCMP_ACT_ALLOW"}`,
		`{"names":["clone"],"action":"SCMP_ACT_ALLOW","args":[{"index":0,"value":2114060288,"op":"SCMP_CMP_MASKED_EQ"}]}`,
		`{"names":["clone"],"action":"SCMP_ACT_ALLOW","args":[{"index":0,"value":2114060416,"valueTwo":128,"op":"SCMP_CMP_MASKED_EQ"}]}`,
	} {
		src := `{"defaultAction":"SCMP_ACT_ERRNO","syscalls":[{"names":["socket"],"action":"SCMP_ACT_ALLOW","args":[{"index":0,"value":1,"op":"SCMP_CMP_EQ"}]},` + rule + `]}`
		_ = os.WriteFile(p.config.SeccompProfile, []byte(src), 0600)
		if _, e := NewPodman(p.config); e == nil {
			t.Fatal("seccomp profile allowed IP, external handler or namespace creation")
		}
	}
	src := `{"defaultAction":"SCMP_ACT_ERRNO","syscalls":[{"names":["socket","socketpair"],"action":"SCMP_ACT_ALLOW","args":[{"index":0,"value":1,"op":"SCMP_CMP_EQ"}]},{"names":["clone"],"action":"SCMP_ACT_ALLOW","args":[{"index":0,"value":2114060416,"valueTwo":0,"op":"SCMP_CMP_MASKED_EQ"}]}]}`
	_ = os.WriteFile(p.config.SeccompProfile, []byte(src), 0600)
	if _, e := NewPodman(p.config); e != nil {
		t.Fatal("bounded Unix/thread seccomp profile denied")
	}
}

func TestSupervisorExpiryCancellationAndSafeTeardown(t *testing.T) {
	for _, mode := range []string{"expiry", "cancel", "success", "broker_deny"} {
		t.Run(mode, func(t *testing.T) {
			p := podmanFixture(t)
			ticket := testTicket()
			if mode == "expiry" {
				ticket.ExpiresAt = time.Now().Add(30 * time.Millisecond)
			}
			f := successfulControl(ticket)
			if mode == "cancel" {
				f.active = false
			}
			if mode == "broker_deny" {
				f.executeError = ErrDenied
			}
			var commands [][]string
			var source string
			p.command = func(ctx context.Context, args []string, limit int) ([]byte, error) {
				commands = append(commands, append([]string(nil), args...))
				if args[0] == "rm" {
					if ctx.Err() != nil {
						t.Error("cleanup uses canceled context")
					}
					return nil, nil
				}
				if limit != 0 {
					t.Error("untrusted stdout captured")
				}
				for _, arg := range args {
					if strings.HasPrefix(arg, "--volume=") {
						source = strings.Split(strings.TrimPrefix(arg, "--volume="), ":")[0]
					}
				}
				if mode == "success" || mode == "broker_deny" {
					return nil, ExecuteConnector(ctx, filepath.Join(source, "request.json"), filepath.Join(source, "execute.sock"), io.Discard)
				}
				<-ctx.Done()
				return nil, ErrUnavailable
			}
			s, _ := NewSupervisor(p, f)
			s.pollInterval = 2 * time.Millisecond
			receipt, e := s.execute(context.Background(), ticket)
			if mode == "success" {
				if e != nil || receipt.Outcome != "succeeded" {
					t.Fatal("safe success receipt missing")
				}
			} else if e == nil {
				t.Fatal("stopped attempt reported success")
			}
			if len(commands) != 2 || commands[1][0] != "rm" || !strings.Contains(strings.Join(commands[1], " "), "--force --ignore --time=0") {
				t.Fatal("force teardown missing")
			}
			if _, e := os.Stat(source); !os.IsNotExist(e) {
				t.Fatal("attempt directory retained")
			}
			if mode != "success" && mode != "broker_deny" && f.executeCalls.Load() != 0 {
				t.Fatal("expiry/cancel dispatched")
			}
		})
	}
}

// This helper uses only a Unix socket. It verifies that transport destinations
// cannot be chosen by a request or proxy environment.
func TestConnectorTransportUsesFixedUnixDestination(t *testing.T) {
	ticket := testTicket()
	dir := t.TempDir()
	socket := filepath.Join(dir, "socket")
	l, e := net.Listen("unix", socket)
	if e != nil {
		t.Fatal(e)
	}
	defer l.Close()
	s := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/execute" || r.Host != "relay" {
			t.Error("unexpected destination")
		}
		_ = json.NewEncoder(w).Encode(ExecuteResponse{Result: json.RawMessage(`{}`), Outcome: "succeeded", ReceiptID: uuid.New()})
	})}
	go func() { _ = s.Serve(l) }()
	defer s.Close()
	b, _ := json.Marshal(ticket.Request())
	file := filepath.Join(dir, "request")
	_ = os.WriteFile(file, b, 0400)
	t.Setenv("HTTP_PROXY", "http://127.0.0.1:1")
	if ExecuteConnector(context.Background(), file, socket, io.Discard) != nil {
		t.Fatal("Unix connector inherited proxy")
	}
}
