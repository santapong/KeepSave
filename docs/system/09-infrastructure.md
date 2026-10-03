# 9. Infrastructure

Part of the [system documentation](README.md), reconciled 2026-10-02.

CI/container builds pin Go 1.27.1 and Node 24.21.0; exact image digests are in the
Dockerfiles. The backend module minimum is now Go 1.26.0. The runtime backend
image is distroless nonroot and includes API, trusted worker and explicit
operator/recovery binaries; only API is the default entrypoint. The frontend
uses nginx-unprivileged. Exact pins and passing image builds do not substitute
for vulnerability, deployment or runtime acceptance.

`docker-compose.yml` is disposable development, not production. It publishes
API8080/frontend3002 and carries known development key/default settings. Production
rejects those keys and wildcard/insecure settings. Never use the test database
or development recovery material for a team installation.

The new [self-hosted reference](../../deploy/self-hosted/README.md) describes
same-origin application TLS at `app.keepsave.draveniq.dev`: Caddy reverse proxy,
frontend, API, PostgreSQL and separate trusted backup worker on the control host.
Environment/key files are outside the repository; private backup storage has
explicit ownership/permissions. Only TLS exposes ports. The API trusts the
specified proxy address, database verification uses configured CA material and
public metrics are denied. Config/schema validation used synthetic files without
starting a service; no DNS, certificates or deployment are claimed.

Marketing remains a separately deployed static site at `keepsave.draveniq.dev`.
The older split-host ADR0016 is superseded for the core application by
[ADR0028](../adr/0028-core-identity-and-vault-release.md). Existing Helm material
is compatibility source, not proof of two-replica operation, current session/
vault cutover or enforced runner network policy. Kubernetes is not required for
the first team installation.

The approved M5 topology adds a connector supervisor on a separate Linux runner
host. It has not been delivered. M5 also requires measured restart/outage/
upgrade/recovery and two-API checks, database-backed admission limits, durable
webhooks and operator runbooks. No throughput, uptime or recovery target is
advertised before measurement.

CI retains race/vet/format/coverage/fuzz/dependency, frontend, container, SAST,
CodeQL and Robot checks for main/develop changes. Source workflow checks are not
remote CI evidence. Exact executed local gates and tool failures belong in the
[acceptance ledger](../validation/2026-10-01-core-release/ACCEPTANCE.md).
