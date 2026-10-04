# 9. Infrastructure and deployment

Part of the [system documentation](README.md). Source reconciled October 4, 2026.

## Runtime pins and topology

CI/container builds pin Go **1.27.1** and Node **24.21.0**; the Go module minimum is
1.26.0. Exact base image pins are in the Dockerfiles. The API runtime is distroless
nonroot, with explicit worker/operator/recovery/harness commands alongside the
server entrypoint. The frontend runs nginx-unprivileged. Current image/scan
receipts are in [OPERATIONS](../validation/2026-10-02-harness-neutral-platform/OPERATIONS.md);
nonroot/pinned images do not establish a clean vulnerability inventory.

| Deployment surface | Purpose and current status |
|---|---|
| Development Compose | Disposable local API/frontend/PostgreSQL; API8080/frontend3002, known development defaults. |
| Static landing | Separately published `keepsave.draveniq.dev`, retaining Field Twist / Event Horizon identity. |
| Control host reference | Same-origin `app.keepsave.draveniq.dev`: TLS proxy, frontend, API, PostgreSQL, trusted worker, private storage and Vault Transit. Not production deployed/accepted. |
| Separate runner reference | Enrolled rootless Podman supervisor on another Linux host, private mTLS connection and digest-pinned Unix-relay connectors. Actual isolation unqualified. |
| Legacy Helm/split-host material | Historical/compatibility reference, not current multi-instance or security acceptance. |

The [control-host bundle](../../deploy/self-hosted/README.md) exposes TLS rather
than database/API ports and keeps operator key/environment files outside Git.
The optional private-listener overlay binds an explicit private IP, verifies runner
client certificates and does not proxy runner authority through public forwarded
headers. The [runner reference](../../deploy/runner/README.md) has no database or
vault mount. Kubernetes is not required for the first team installation.

## Runner enforcement requirements

The supported host must prove rootless Podman, cgroups v2 CPU/memory/PID delegation
and seccomp. Connector reference limits are read-only root, no IP network, 64 MiB
scratch, one CPU, 256 MiB memory and 32 processes. Only an attempt-specific Unix
relay is mounted; the supervisor's identity/engine access stays outside. The
current development host lacks CPU delegation and preflight refuses execution.
A successful preflight alone would still not prove file/network/resource attacks
are contained; those actual host tests remain gates.

## Configuration and availability

New identity/team/MCP/run/admission/dispatch flags default false. SMTP acceptance
and recovery acceptance are separate operator records. Production startup refuses
known development keys, short signing secrets, wildcard/insecure origins and
insecure PostgreSQL settings. Vault Transit is the reference wrapping-key source;
recovery material must be independently retained. Configuration/schema checks use
synthetic files and do not create DNS, certificates, provider apps or services.

[ADR0028](../adr/0028-core-identity-and-vault-release.md) and
[ADR0029](../adr/0029-harness-neutral-platform.md) supersede hosted/single-harness
planning for the current candidate. Full worker/supervisor/database/Transit faults,
two-API deployment, upgrade/compatible rollback, isolated key/storage recovery
and measured fixed-hardware capacity remain unaccepted. October 3 main/develop
CI runs were checked failed on October 4; local passing receipts are not green
remote CI. See [testing](11-testing.md) and the [publication note](../releases/2026-10-03-source-publication.md).
