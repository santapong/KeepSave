# KeepSave — Architecture Dependency Map

One-page diagram of which package depends on which, what crosses each boundary, and where the trust boundaries are. The point is to make blast radius visible: if a component is compromised or replaced, you can read this page and know what else needs to be reviewed.

This is a *living* doc — when a new package is added or a boundary moves, update it in the same PR.

---

## High-level flow

```
                         ┌─────────────────────────────┐
                         │       HTTP client           │
                         │  (dashboard │ embed widget  │
                         │   │ CLI │ AI agent)         │
                         └──────────────┬──────────────┘
                                        │   HTTPS
                                        ▼
══════════════════════ trust boundary 1: network ═════════════════════
                                        │
                         ┌──────────────▼──────────────┐
                         │   backend/internal/api      │
                         │   (Gin handlers + mux)      │
                         └──────────────┬──────────────┘
                                        │
                         ┌──────────────▼──────────────┐
                         │   middleware                │
                         │   • JWTAuthMiddleware       │
                         │   • APIKeyAuthMiddleware    │ ← ADR-0002
                         │   • request validation      │
                         │   • audit-log enrichment    │
                         └──────────────┬──────────────┘
                                        │
══════════ trust boundary 2: authenticated caller identity ══════════
                                        │
                         ┌──────────────▼──────────────┐
                         │   service                   │
                         │   • secret_service          │
                         │   • project_service         │
                         │   • promotion_service       │ ← ADR-0003
                         │   • intelligence/AI         │
                         └────┬──────────┬─────────────┘
                              │          │
                ┌─────────────▼──┐   ┌───▼─────────────────┐
                │   crypto       │   │   repository        │
                │   (AES-256-GCM)│   │   (PostgreSQL)      │
                │   ← ADR-0001   │   │                     │
                └────────┬───────┘   └──────────┬──────────┘
                         │                      │
                ┌────────▼──────────┐           │
                │   keyprovider     │           │
                │   • EnvProvider   │           │
                │   • KMS (AWS/GCP) │ ← ADR-0004│
                └────────┬──────────┘           │
                         │                      │
══════════ trust boundary 3: storage / key custody ══════════════════
                         │                      │
                         ▼                      ▼
                   ┌──────────┐           ┌──────────────┐
                   │  KMS /   │           │  PostgreSQL  │
                   │  env var │           │  (encrypted  │
                   │          │           │   at rest    │
                   │          │           │   via crypto)│
                   └──────────┘           └──────────────┘
```

## Package dependency direction

The arrow direction is "depends on" — `api` calls `service`, `service` calls `crypto` and `repository`, and so on. **There are no upward edges.** Crypto does not call service. Repository does not call middleware. If you find yourself adding a back-edge, stop and re-think — it's the most reliable smell that a boundary is being violated.

```
   cmd/server        (composition root)
       │
       ▼
   internal/api      (handlers + middleware + router)
       │
       ▼
   internal/service  (business logic)
       │
       ├──► internal/crypto  ──► internal/crypto/keyprovider
       │
       ├──► internal/repository
       │
       ├──► internal/auth
       │
       ├──► internal/events
       │
       └──► internal/promotion (if separated)

   internal/models   ← imported by everyone, depends on nothing
   internal/logging  ← imported by everyone, depends on nothing
   internal/metrics  ← imported by api + service, depends on nothing
   internal/tracing  ← imported by api + service, depends on nothing
   internal/config   ← imported by cmd/server only
```

## Trust boundaries

There are three, and crossing each is a security event:

1. **Network boundary** — anything from outside the backend. Everything entering is hostile until proven otherwise. Handled by TLS termination + the auth middlewares + request validation. *Failure mode:* unauthenticated callers reach service-layer code.

2. **Caller-identity boundary** — once a request has been authenticated (`user_id` set in context), downstream code trusts that identity. *Failure mode:* IDOR — authenticated user accessing another user's data. Mitigated by per-handler authorization checks; **every handler is its own choke point**.

3. **Key-custody boundary** — the master key lives outside the database; the DB only ever holds DEKs encrypted under the master and secrets encrypted under DEKs. *Failure mode:* master key written to disk in a log, copied to a backup, or exfiltrated from process memory. Mitigated by `keyprovider` interface keeping the master out of normal control flow.

## Cross-cutting concerns

These imports are universal and don't form part of the layered dependency:

- **`internal/models`** — pure data types, no behavior. Imported by every layer. Adding behavior here is a smell.
- **`internal/logging`** — structured logging. **Must redact secrets** on every call. There is a known risk of accidentally logging a plaintext value; this is an open item for the Backend 30-day error-wrapping work.
- **`internal/metrics`** — Prometheus-format counters / gauges / histograms. **Never label by anything user-controlled** (project name, secret key) — that creates unbounded label cardinality and is also a privacy leak.
- **`internal/tracing`** — distributed tracing spans. Same redaction rule as logging.

## Frontend (separate process, separate trust domain)

```
   frontend/src/
       ├── pages/         (React Router routes)
       ├── components/    (UI primitives + composites)
       ├── hooks/         (data fetching, auth state)
       ├── api/           (typed client to backend)
       └── embed/         (Web Component widget) ← own trust boundary,
                                                   runs on integrator origin
```

The **embed widget** runs on third-party origins. It is its own trust domain: any DOM access, postMessage, or storage call must assume the host page is hostile. State machine and origin-allowlist documented in the 30-day Frontend work (`docs/EMBED_STATE.md` — to be created).

## Database boundary

PostgreSQL is treated as **untrusted at rest**: all sensitive columns are encrypted before insertion. The schema lives in `backend/migrations/`. Promotion does not bypass the encryption layer — see ADR-0003.

## Hot paths

- **Secret read (most frequent):** `api → middleware (auth) → service.secret_service → repository (fetch encrypted) → crypto (unwrap DEK, decrypt secret) → response`. Tight loop; performance regressions here are user-visible.
- **Promotion (highest blast radius):** `api → middleware → service.promotion_service → service.secret_service (loop) → repository (loop)`. Slow path by design; correctness over speed.
- **Master key startup:** `cmd/server → keyprovider → service initialization`. Runs once at boot; failure is fatal and *should* be — silent fallback to a default key would be a security disaster.

## What is *not* on this page (and why)

- **External integrations** (Seidr, MedQCNN, Nexus) live under `integrations/`. They are out-of-process and treated as untrusted clients to KeepSave — they enter through the same auth boundary as any other caller. Their internal structure is irrelevant to this map.
- **The dashboard's internal component tree.** That's frontend architecture; if it grows complex enough to need a map, give it its own file in `frontend/docs/`.

---

**When to update this file:** any time a new top-level package is added under `backend/internal/`, a new trust boundary is introduced, or a dependency direction changes. Treat it like a header file: out-of-date is worse than missing.
