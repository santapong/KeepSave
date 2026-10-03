package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
)

const podmanBinary = "/usr/bin/podman"

// Linux CLONE_NEWNS/CGROUP/UTS/IPC/USER/PID/NET/TIME. Threads may be
// created by the Go runtime, but this connector cannot create namespaces.
const namespaceCloneMask = 0x7e020080

type Config struct{ ImageDigest, AttemptRoot, SeccompProfile string }
type Diagnostic struct {
	Available         bool   `json:"available"`
	Reason            string `json:"reason"`
	IsolationAccepted bool   `json:"isolation_accepted"`
}
type commandFunc func(context.Context, []string, int) ([]byte, error)
type Podman struct {
	config  Config
	command commandFunc
}

// NewPodman permits only the installed local Podman binary. There is no remote
// daemon, arbitrary command/image builder or request-selected execution path.
func NewPodman(config Config) (*Podman, error) {
	if !ValidImage(config.ImageDigest) || !secureDirectory(config.AttemptRoot) || !secureProfile(config.SeccompProfile) {
		return nil, ErrInvalid
	}
	return &Podman{config: config, command: localCommand}, nil
}

func secureDirectory(path string) bool {
	if !safeAbsolute(path) || len(path) > 64 {
		return false
	}
	info, e := os.Lstat(path)
	if e != nil || !info.IsDir() || info.Mode().Perm() != 0700 {
		return false
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	return ok && int(st.Uid) == os.Getuid()
}

func safeAbsolute(path string) bool {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path || len(strings.Split(path, "/")) < 3 || strings.ContainsAny(path, ":,\x00\r\n") {
		return false
	}
	// Symlink ancestors would make the sole mount target ambiguous.
	for p := path; p != "/"; p = filepath.Dir(p) {
		info, e := os.Lstat(p)
		if e != nil || info.Mode()&os.ModeSymlink != 0 {
			return false
		}
	}
	return true
}

func secureProfile(path string) bool {
	if !safeAbsolute(path) {
		return false
	}
	info, e := os.Lstat(path)
	if e != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0022 != 0 || info.Size() > 64*1024 {
		return false
	}
	b, e := os.ReadFile(path)
	if e != nil {
		return false
	}
	var profile struct {
		DefaultAction string `json:"defaultAction"`
		Syscalls      []struct {
			Names  []string `json:"names"`
			Action string   `json:"action"`
			Args   []struct {
				Index    int    `json:"index"`
				Value    int    `json:"value"`
				ValueTwo int    `json:"valueTwo"`
				Op       string `json:"op"`
			} `json:"args"`
		} `json:"syscalls"`
	}
	if json.Unmarshal(b, &profile) != nil || profile.DefaultAction != "SCMP_ACT_ERRNO" {
		return false
	}
	unixOnly := false
	for _, call := range profile.Syscalls {
		// LOG/TRACE/NOTIFY are not isolation denials and require an external
		// handler. Do not let an alternate rule bypass the checked allowlist.
		if call.Action != "SCMP_ACT_ALLOW" && call.Action != "SCMP_ACT_ERRNO" {
			return false
		}
		for _, name := range call.Names {
			if call.Action != "SCMP_ACT_ALLOW" {
				continue
			}
			if name == "socket" || name == "socketpair" {
				only := false
				for _, arg := range call.Args {
					if arg.Index == 0 && arg.Value == 1 && arg.Op == "SCMP_CMP_EQ" {
						only = true
					}
				}
				if !only {
					return false
				}
				if name == "socket" {
					unixOnly = true
				}
			}
			if name == "clone" {
				bounded := false
				for _, arg := range call.Args {
					if arg.Index == 0 && arg.Op == "SCMP_CMP_MASKED_EQ" && arg.Value&namespaceCloneMask == namespaceCloneMask && arg.ValueTwo&namespaceCloneMask == 0 {
						bounded = true
					}
				}
				if !bounded {
					return false
				}
			}
			if name == "bpf" || name == "mount" || name == "setns" || name == "unshare" || name == "ptrace" || name == "keyctl" || name == "io_uring_setup" || name == "clone3" {
				return false
			}
		}
	}
	return unixOnly
}

type cappedWriter struct {
	bytes.Buffer
	remaining int
	exceeded  bool
}

func (w *cappedWriter) Write(p []byte) (int, error) {
	n := len(p)
	if n > w.remaining {
		w.exceeded = true
		p = p[:w.remaining]
	}
	w.remaining -= len(p)
	_, _ = w.Buffer.Write(p)
	return n, nil
}

func localCommand(ctx context.Context, args []string, limit int) ([]byte, error) {
	cmd := exec.CommandContext(ctx, podmanBinary, args...)
	// Podman needs the rootless user's configured storage location/runtime dir;
	// allow only these OS substrate values, never inherited provider credentials.
	cmd.Env = []string{"PATH=/usr/bin:/bin", "LANG=C"}
	for _, key := range []string{"HOME", "XDG_RUNTIME_DIR", "DBUS_SESSION_BUS_ADDRESS"} {
		if val, ok := os.LookupEnv(key); ok {
			cmd.Env = append(cmd.Env, key+"="+val)
		}
	}
	cmd.Stdin = nil
	cmd.Stderr = io.Discard
	if limit == 0 {
		cmd.Stdout = io.Discard
		if cmd.Run() != nil {
			return nil, ErrUnavailable
		}
		return nil, nil
	}
	w := &cappedWriter{remaining: limit}
	cmd.Stdout = w
	if cmd.Run() != nil || w.exceeded {
		return nil, ErrUnavailable
	}
	return w.Bytes(), nil
}

func (p *Podman) Preflight(ctx context.Context) Diagnostic {
	deny := func(reason string) Diagnostic { return Diagnostic{Reason: reason} }
	if runtime.GOOS != "linux" || os.Getuid() == 0 {
		return deny("rootless_linux_required")
	}
	if !secureDirectory(p.config.AttemptRoot) || !secureProfile(p.config.SeccompProfile) {
		return deny("private_configuration_required")
	}
	b, e := p.command(ctx, []string{"info", "--format=json"}, 64*1024)
	if e != nil {
		return deny("podman_unavailable")
	}
	var info struct {
		Host struct {
			OS                string   `json:"os"`
			CgroupVersion     string   `json:"cgroupVersion"`
			CgroupControllers []string `json:"cgroupControllers"`
			ServiceIsRemote   bool     `json:"serviceIsRemote"`
			Security          struct {
				Rootless       bool `json:"rootless"`
				SeccompEnabled bool `json:"seccompEnabled"`
			} `json:"security"`
		} `json:"host"`
	}
	if json.Unmarshal(b, &info) != nil || info.Host.OS != "linux" || !info.Host.Security.Rootless || info.Host.ServiceIsRemote {
		return deny("rootless_local_podman_required")
	}
	if info.Host.CgroupVersion != "v2" {
		return deny("cgroup_v2_required")
	}
	if !info.Host.Security.SeccompEnabled {
		return deny("seccomp_required")
	}
	for _, need := range []string{"cpu", "memory", "pids"} {
		found := false
		for _, controller := range info.Host.CgroupControllers {
			if controller == need {
				found = true
			}
		}
		if !found {
			return deny("cgroup_" + need + "_missing")
		}
	}
	help, e := p.command(ctx, []string{"run", "--help"}, 64*1024)
	if e != nil {
		return deny("podman_capabilities_unavailable")
	}
	for _, flag := range []string{"--read-only-tmpfs", "--unsetenv-all", "--timeout", "--userns", "--pids-limit", "--cpus", "--security-opt", "--image-volume", "--log-driver", "--http-proxy"} {
		if !bytes.Contains(help, []byte(flag)) {
			return deny("podman_control_unsupported")
		}
	}
	rmHelp, e := p.command(ctx, []string{"rm", "--help"}, 64*1024)
	if e != nil || !bytes.Contains(rmHelp, []byte("--ignore")) || !bytes.Contains(rmHelp, []byte("--force")) || !bytes.Contains(rmHelp, []byte("--time")) {
		return deny("podman_teardown_unsupported")
	}
	b, e = p.command(ctx, []string{"image", "inspect", "--", p.config.ImageDigest}, 64*1024)
	if e != nil {
		return deny("approved_image_missing")
	}
	var images []struct {
		RepoDigests []string `json:"RepoDigests"`
		Config      struct {
			Volumes map[string]any    `json:"Volumes"`
			Labels  map[string]string `json:"Labels"`
		} `json:"Config"`
	}
	if json.Unmarshal(b, &images) != nil || len(images) != 1 || len(images[0].Config.Volumes) > 0 {
		return deny("approved_image_invalid")
	}
	match := false
	for _, digest := range images[0].RepoDigests {
		if digest == p.config.ImageDigest {
			match = true
		}
	}
	if !match {
		return deny("approved_image_digest_mismatch")
	}
	return Diagnostic{Available: true, Reason: "capability_preflight_passed", IsolationAccepted: false}
}

func containerName(t Ticket) string { return "keepsave-attempt-" + t.TicketID.String() }

func (p *Podman) arguments(t Ticket, dir string) ([]string, error) {
	if t.ImageDigest != p.config.ImageDigest || !ValidImage(t.ImageDigest) || !safeAbsolute(dir) || filepath.Dir(dir) != p.config.AttemptRoot {
		return nil, ErrInvalid
	}
	return []string{"run", "--rm", "--name=" + containerName(t), "--pull=never", "--network=none", "--read-only", "--read-only-tmpfs=false", "--image-volume=ignore", "--tmpfs=/scratch:rw,noexec,nosuid,nodev,size=64m,mode=0700,uid=65532,gid=65532", "--workdir=/scratch", "--cpus=1", "--memory=256m", "--memory-swap=256m", "--pids-limit=32", "--cap-drop=ALL", "--security-opt=no-new-privileges", "--security-opt=seccomp=" + p.config.SeccompProfile, "--userns=keep-id:uid=65532,gid=65532", "--user=65532:65532", "--pid=private", "--ipc=none", "--uts=private", "--cgroupns=private", "--http-proxy=false", "--unsetenv-all", "--no-hosts", "--dns=none", "--hostname=keepsave-connector", "--no-healthcheck", "--systemd=false", "--log-driver=none", "--timeout=30", "--stop-timeout=0", "--umask=0077", "--entrypoint=/keepsave-connector", "--volume=" + dir + ":/relay:ro,nosuid,nodev,noexec", t.ImageDigest, "--request=/relay/request.json", "--socket=/relay/execute.sock"}, nil
}
