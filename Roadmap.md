# KeepSave Roadmap

## Vision

A secure, self-hosted vault for environment variables with a controlled promotion pipeline (Alpha -> UAT -> PROD) and an embeddable frontend widget that integrates into any website.

---

## Phase 1 - Foundation (Core Backend + Storage)

**Goal:** Encrypted secret storage with CRUD API and basic auth.

### Backend
- [x] Project scaffold (Go + Gin, PostgreSQL, Docker Compose)
- [x] Database schema: `projects`, `environments`, `secrets`, `audit_log`
- [x] AES-256-GCM encryption layer with envelope encryption
- [x] CRUD API for projects (`POST/GET/PUT/DELETE /api/v1/projects`)
- [x] CRUD API for secrets (`POST/GET/PUT/DELETE /api/v1/projects/:id/secrets`)
- [x] Environment scoping (Alpha, UAT, PROD per project)
- [x] JWT authentication (register/login)
- [x] API key generation for AI Agent access
- [x] Request validation and error handling
- [x] Unit tests for crypto and service layers

### Deliverables
- Running API that stores and retrieves encrypted secrets
- API key auth so an AI Agent can read secrets without exposing them in code

---

## Phase 2 - Environment Promotion Engine

**Goal:** Move secrets safely between Alpha -> UAT -> PROD.

### Backend
- [x] Promotion endpoint (`POST /api/v1/projects/:id/promote`)
- [x] Promotion rules engine (which keys to promote, override policy)
- [x] Diff view: show what will change in target environment
- [x] Audit log for every promotion event
- [x] Rollback support (restore previous secret set)
- [x] Optional approval workflow for PROD promotions
- [x] Integration tests for promotion flows

### Deliverables
- One-command promotion of secrets between environments
- Full audit trail of who promoted what and when

---

## Phase 3 - Frontend Dashboard

**Goal:** Web UI for managing projects, secrets, and promotions.

### Frontend
- [x] Project scaffold (React 18 + TypeScript + Vite)
- [x] Auth pages (login, register)
- [x] Project list and detail views
- [x] Secret management UI (add, edit, mask/reveal, delete)
- [x] Environment switcher (Alpha / UAT / PROD tabs)
- [x] Promotion wizard (select env, review diff, confirm)
- [x] Audit log viewer
- [x] API key management page
- [x] Responsive layout

### Deliverables
- Fully functional dashboard to manage secrets and promotions via browser

---

## Phase 4 - Embeddable Widget

**Goal:** A `<keepsave-widget>` Web Component that third-party sites can embed.

### Frontend (Embed SDK)
- [ ] Web Component shell using Shadow DOM
- [ ] Configurable via HTML attributes (`project-id`, `api-url`, `theme`)
- [ ] Auth flow via postMessage handshake with parent page
- [ ] Read-only and read-write modes
- [ ] Theming support (light/dark, custom CSS variables)
- [ ] NPM package + CDN script for easy integration
- [ ] Integration guide and example host page

### Deliverables
- Single `<script>` tag inclusion for any website
- Isolated widget that does not leak styles or clash with host page

---

## Phase 5 - Hardening and Operations

**Goal:** Production-grade reliability and security.

- [ ] Rate limiting and request throttling
- [ ] Key rotation mechanism (re-encrypt secrets with new master key)
- [ ] Secret versioning (keep N previous values)
- [ ] Webhook notifications on promotion events
- [ ] Health check and readiness endpoints
- [ ] Structured JSON logging
- [ ] Helm chart for Kubernetes deployment
- [ ] CI/CD pipeline (GitHub Actions: lint, test, build, push image)
- [ ] Security audit checklist (OWASP top 10 review)

---

## Phase 6 - Advanced Features (Future)

- [ ] Multi-tenant SaaS mode with organization accounts
- [ ] RBAC with custom roles (viewer, editor, admin, promoter)
- [ ] Secret templates (predefined sets for common stacks)
- [ ] CLI tool (`keepsave pull`, `keepsave push`, `keepsave promote`)
- [ ] Integration plugins (GitHub Actions, GitLab CI, Terraform provider)
- [ ] Import/export from `.env` files
- [ ] Secrets dependency graph (detect references between secrets)

---

## Milestone Summary

| Phase | Name                  | Key Outcome                              |
|-------|-----------------------|------------------------------------------|
| 1     | Foundation            | Encrypted secret storage + API key auth  |
| 2     | Promotion Engine      | Alpha -> UAT -> PROD pipeline            |
| 3     | Frontend Dashboard    | Full web UI for secret management        |
| 4     | Embeddable Widget     | `<keepsave-widget>` for third-party sites |
| 5     | Hardening             | Production readiness                     |
| 6     | Advanced Features     | CLI, RBAC, SaaS, integrations            |
