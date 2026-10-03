# Core control-host deployment reference

This is a reviewable **reference bundle**, separate from development Compose.
It has not been deployed or subjected to production availability/load/upgrade
acceptance. Independent review and the acceptance ledger remain release gates.
It does not deliver M3 runner isolation or complete M5 operations.

The application shares `https://app.keepsave.draveniq.dev` for SPA, API and
provider callbacks. The existing marketing origin remains unchanged. Only the
TLS service publishes ports. PostgreSQL, API, frontend and trusted worker have
no host ports. Public metrics are refused at the proxy. The proxy overwrites
forwarded client information; the API trusts only its explicit private IP.
Change the proxy IP and subnet together if they overlap operator networks.

The trusted worker runs local audit publication, lifecycle reminders, proof delivery (only after SMTP acceptance), expiration maintenance and outbox/backup jobs. It receives control-host
key/database authority and private backup storage; it is not a connector
runner. The implemented runner reference requires its own host and a restricted mTLS broker relay. See [runner reference](../runner/README.md) and [current setup](../../docs/design/2026-10-02-harness-neutral-platform/SETUP.md).

## Operator prerequisites

- Complete independent review, choose a pinned application image from accepted
  source, and retain an external backup plus independently recoverable key.
- Register the application DNS/TLS endpoint and Google/GitHub callback URLs.
  This reference does not create provider applications, DNS records or secrets.
- Supply separate runtime/database env files **outside the checkout** with
  mode0600. Examples contain placeholders only; the production file has no
  fallback passwords, master key or signing secret. Vault Transit is the
  production reference key source. Preserve its key versions and wrapped root.
- Supply PostgreSQL `server.crt` and `server.key` with a certificate valid for
  hostname `db`; key ownership/permissions must satisfy the PostgreSQL image.
  Supply its CA file to API/worker. The runtime DSN uses `sslmode=verify-full`.
- Create private backup and TLS storage directories, owned by uid/gid65532;
  backups require mode0700. Keep backups outside the repository and outside
  the frontend served tree. Copy verified bundles to independent private storage
  as part of the operator recovery policy; local storage alone is not disaster recovery.

Set these Compose interpolation variables from a separate operator shell/config,
without printing resolved configuration or secret files:

```text
KEEPSAVE_RUNTIME_ENV_FILE=<absolute private runtime env path>
KEEPSAVE_DATABASE_ENV_FILE=<absolute private database env path>
KEEPSAVE_DATABASE_TLS_DIRECTORY=<absolute certificate directory>
KEEPSAVE_DATABASE_CA_FILE=<absolute CA certificate path>
KEEPSAVE_BACKUP_DIRECTORY=<absolute private backup directory>
KEEPSAVE_TLS_DATA_DIRECTORY=<absolute private TLS storage directory>
KEEPSAVE_RECOVERY_VERIFIED=false
```

`docker compose -f deploy/self-hosted/compose.yml config --quiet` validates
structure without starting services. Running `up` publishes the application and
starts certificate issuance, so execute deployment only under separate operator
approval. Caddy's internal ports8080/8443 map to public80/443 as described in its
[port documentation](https://caddyserver.com/docs/caddyfile/options#http-port).

## Coordinated existing-database cutover

1. Drain all old API/worker writers and keep the recovery backup/key outside the
   target. Apply the additive migration chain with one controlled process;
   concurrent migrations are not an accepted procedure.
2. Before opening traffic, run the new image's `/app/keepsave-vault -action
   baseline` explicitly on the trusted host against the intended database.
   It validates encrypted current/snapshot records and adds a labeled baseline,
   without claiming unavailable historical revisions. Startup refuses active
   unenrolled projects. Migration023 retains project/audit identities,
   024 binds promotion source artifacts, and025 adds the backup metadata catalog.
3. Start the new API/frontend only after successful enrollment and readiness.
   Do not restart older writers on the enrolled vault. Verify login, scoped
   read, one new revision and an encrypted backup before admitting the team.
4. Enable the worker with `KEEPSAVE_RECOVERY_VERIFIED=false` to process local
   events. Perform an external-file verification and fresh isolated recovery
   drill using the trusted CLI and independent recovery material.
5. Set `KEEPSAVE_RECOVERY_VERIFIED=true` only after recording that drill. Daily
   encrypted backups become due after02:00UTC; a deterministic day/job ID prevents
   duplicates. Retention keeps 30 scheduled copies and at least two verified
   copies; manual/pre-upgrade bundles are held separately. Failed verification,
   audit or file operations remain visible rather than being counted as success.

The binaries are included in the API image but are never HTTP routes:
`keepsave-operator` grants/revokes a verified immutable user ID;
`keepsave-vault` baselines, verifies and recovers into an explicitly fresh
isolated PostgreSQL database without non-system tables; `keepsave-worker` executes local durable work. Alternate
entrypoint commands must use the correct private operator configuration.

The isolated recovery command refuses existing non-system tables before any
migration and holds a database-scoped operator lock while recovering. It creates
new project/record identities; source users, memberships, sessions, API keys, leases
and run authority are excluded. Use metadata previews and selected revision
checks for live restoration. Deleted secrets/projects are not undeleted.

## Operational limits

This bundle is one control host. It does not prove two-replica revocation,
coordination under migration contention, throughput, recovery time, rollback of
unsupported journal formats or hardware failure. Do not advertise those results
before their recorded exercises. Preserve key/history dependencies and do not
purge ciphertext or keys speculatively. Revocation/status/audit/recovery stay
available when future run admission is switched off.

Keep local backup storage private, monitor durable job/catalog states, retain
external copies and regularly repeat the isolated recovery drill. Never rollback
an old binary onto an enrolled vault or restore source authorization tables from
a vault bundle.

## Optional isolated-runner listener

The additive `runner-listener.compose.yml` exposes only the private direct mTLS
listener. Set an explicit private `KEEPSAVE_PRIVATE_CONTROL_IP` and a restricted
`KEEPSAVE_RUNNER_SERVER_TLS_DIRECTORY`; never route it through public Caddy.
The API requires verified client certificates and enrolled fingerprints. Validate
with both Compose files and `config --quiet`; deployment/enrollment still requires
operator resources and acceptance. New identity/team/MCP/run flags default off.
Admission and dispatch can be disabled while authorized status/cancel/revocation
remain available. Real SMTP/GitHub/client/host acceptance is separate.

The runtime worker requires canonical application origin for proof links. It
receives no runner-listener key; mount runner TLS only into the API. Version2
recovery requires explicit lifecycle custodian mapping; legacy version1 remains
a reader contract. Do not use incompatible older readers/writers for rollback.
