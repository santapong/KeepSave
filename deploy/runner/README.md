# Isolated connector supervisor — local implementation

Observed 2026-10-02: the focused synthetic Go race suite passed. This reference
has not been deployed, enrolled with operator credentials or used against
GitHub. No connector image was built or pulled in this task. Independent
Security Engineer and Tech Lead review, approved image review and isolated-host
acceptance remain release gates under ADR0029.

## Topology and custody

Install `keepsave-runner` as a dedicated nonroot Linux user on a host separate
from the control API, PostgreSQL and vault. It makes outbound TLS1.3 mutual-TLS
calls to exactly the configured HTTPS control origin; proxy environment and
redirects are refused. The control host enrolls the certificate's DER SHA256
fingerprint and the full `registry/repository@sha256:...` connector digest in a
project under explicit operator approval. A certificate fingerprint identifies
the enrolled supervisor; it is not hardware attestation.

The supervisor has its client certificate/key and trusted server CA. It receives
one short-lived ticket per durable attempt. The random ticket token exists only
in supervisor/relay memory. It is absent from Podman arguments, connector
environment, request files and safe receipts. The runner does not accept DB,
vault, GitHub App or provider credential configuration. The Broker on the
control host retains provider custody and rechecks current authority before
each upstream request.

The connector receives the exact bound operation request in a regular readonly
file, and one per-attempt Unix socket at `/relay/execute.sock`. The sole host
mount is the fresh attempt directory, readonly with `nodev,nosuid,noexec`. The
parent directory is operator-private `0700`; the attempt directory is `0750`,
request file `0440` and socket `0660`. `keep-id:uid=65532,gid=65532` maps the
dedicated supervisor's primary UID/GID to the connector identity. Do not run
unrelated untrusted workloads as that same OS principal or give unrelated users
membership in its primary group. Keep the configured attempt root at most 64
bytes so socket paths fit the Linux Unix-domain limit. It and its ancestors
must not be symlinks; the root must be owned by the running user.

Each relay permits one exactly equal typed request. It refuses unknown or
duplicate JSON fields, trailing JSON, arbitrary URLs/methods, changed run/grant/
attempt/fence/image/digest/nonce/kind/arguments, expired tickets and subsequent
calls. It invokes only `/api/v1/runner/operations/execute` through the
supervisor's mTLS client. The connector sees bounded permitted provider content
and emits only operation/receipt IDs and outcome; the supervisor discards all
connector stdout/stderr and disables Podman logs. Control-host receipts and
durable results determine publication authority.

## Required local controls

`keepsave-runner` requires local `/usr/bin/podman`, rootless Linux, cgroup v2 with
delegated `cpu`, `memory` and `pids` controllers, seccomp, an already installed
approved digest image and supported Podman flags. No builds, pulls, unrestricted
fallback or remote Podman service are permitted. Image-declared volumes are
refused and image environment is cleared. The runtime command sets:

- No IP network, Unix-only socket seccomp rules and no proxy environment.
- Readonly root filesystem with automatic readonly tmpfs disabled.
- One 64MiB `noexec,nosuid,nodev` scratch tmpfs, one CPU, 256MiB RAM/no extra
  swap and 32 PIDs.
- UID/GID65532, all capabilities dropped, no-new-privileges, private PID/IPC/
  UTS/cgroup namespaces, no healthcheck/systemd and no container logs.
- Exactly one per-attempt readonly relay mount; no home, daemon socket, DB,
  vault, service-account token, provider token or mTLS key mount.

The reviewed seccomp reference defaults to errno, restricts `socket/socketpair`
to AF_UNIX and disallows namespace clone flags. It does not claim protection
from a kernel/runtime escape. Its compatibility must be proven on the selected
kernel/architecture/runtime by real acceptance; a syntactically valid profile
does not establish enforcement.

Deadline is the earlier of ticket expiry and 30 seconds. Authority status is
polled through fixed `/api/v1/runner/operations/status` every second with a
two-second request cap. Cancellation, denial, authority outage or expiry cancels
the connector, closes the relay and invokes bounded forced removal. Removal
failure stops the supervisor instead of claiming another operation. A previously
dispatched request may remain uncertain or already complete at the Broker;
teardown cannot recall data returned before revocation.

## Operator installation reference

Build the two Go binaries from reviewed source using the exact Go1.27.1
toolchain. The optional `connector.Dockerfile` has digest-pinned build input and
a scratch runtime, and is built with `backend` as its build context. Only an
operator's separately reviewed build workflow may build/publish/install it.
Record the resulting registry digest and review its source before enrollment;
the supervisor neither creates an image nor accepts a tag.

Prepare a dedicated private directory and copy `seccomp-unix.json` into an
operator-controlled, non-group/world-writable regular file. Keep the client key
in an operator-private regular file with mode0600 or stricter, and issue a
certificate with explicit TLS client-auth usage. Do not put credential material
in shell arguments or environment. The command arguments below name files only.

Diagnostics are the default and do not claim work:

```text
keepsave-runner --image registry.example/keepsave/connector@sha256:<reviewed-digest> \
  --attempt-root /var/lib/keepsave-runner/attempts \
  --seccomp-profile /etc/keepsave-runner/seccomp-unix.json
```

After independent review and actual host acceptance, explicitly enable dispatch:

```text
keepsave-runner --execute \
  --image registry.example/keepsave/connector@sha256:<reviewed-digest> \
  --attempt-root /var/lib/keepsave-runner/attempts \
  --seccomp-profile /etc/keepsave-runner/seccomp-unix.json \
  --control-origin https://control.example \
  --client-cert /etc/keepsave-runner/client.pem \
  --client-key /etc/keepsave-runner/client.key \
  --control-ca /etc/keepsave-runner/control-ca.pem
```

The configured origin contains only scheme and authority; all API paths are
fixed. The supervisor connects directly to the dedicated control API listener, which
terminates mutual TLS and verifies the peer certificate against the configured
client CA. A TCP passthrough may preserve that handshake; an HTTP reverse proxy
that terminates it cannot authenticate this implementation through certificate
headers. Keep ordinary API callers unable to reach this authenticated runner
surface. Configuration and consent remain separate from
this source reference; no operator enrollment has occurred here.

## Evidence and remaining acceptance

The synthetic tests prove DTO/body bounds, binding changes, one-use cardinality,
TLS1.3/mTLS certificate verification, fixed HTTP/Unix destinations, proxy and
redirect refusal, credential-free request files, metadata-only acknowledgements,
expiry/cancellation/broker-denial and cleanup command selection. Their fake
command adapter does not run Podman or prove OS isolation. Every diagnostic
returns `isolation_accepted:false`, even after capability preflight succeeds.

Actual read-only host probe on2026-10-02: Podman5.8.6 reported rootless=true,
cgroupVersion=v2, seccompEnabled=true, serviceIsRemote=false, but its delegated
controllers were only memory/pids. CPU delegation was absent. Therefore the
runner correctly reports unavailable with `cgroup_cpu_missing`; the implementation
must not attempt the container or weaken its one-CPU requirement on this host.
No service/cgroup/host configuration was changed to make this check pass.

Before any operational isolation claim, collect actual evidence on a dedicated
approved host for:

1. Effective cgroup CPU/memory/PID limits, seccomp mode, capabilities, readonly
   filesystem, only the intended64MiB scratch and readonly relay mount.
2. Malicious vetted test image attempts at host-home/DB/vault/daemon/mTLS-key
   reads, AF_INET/AF_INET6/socket egress, mount/namespace escape, fork/memory/CPU
   exhaustion, extra relay calls and oversized request/result bodies.
3. A synthetic token canary kept only at Broker, absent from connector
   environment/files/arguments/stdout/Podman logs/results, including transformed
   leakage attempts. Permitted repository content still has its own data policy.
4. Operator-approved certificate/image enrollment, different-certificate and
   stolen-ticket denial, current membership/grant/run/policy cancellation,
   authority outage, deadline teardown and crash leftovers bounded by Podman's
   independent30s runtime timeout.
5. Actual durable dispatch/result/receipt behavior and uncertainty handling
   through the control host; no blind retry after possible external dispatch.

Independent reviews, real GitHub/harness UAT, operational install and recovery
remain pending. Synthetic passing tests and supported flags establish neither
production readiness nor isolated-host acceptance.
