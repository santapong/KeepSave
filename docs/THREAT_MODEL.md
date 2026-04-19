# KeepSave Threat Model

**Version:** 1.1.0 | **Last review:** 2026-04-19

STRIDE pass on the four highest-risk subsystems. Scope is limited to the
KeepSave service itself; client SDKs and third-party MCP servers are listed
as trust boundaries.

## Trust boundaries

```
[Browser / SDK / MCP client] --(TLS)--> [Gin HTTP layer]
  --> [Auth middleware: JWT or API key]
  --> [Service layer]
  --> [crypto.Service + MasterKeyProvider]
  --> [Postgres]
```

Assets: master key, per-project DEKs, secret plaintext, OAuth client
secrets, API key hashes, audit log, backup snapshots.

---

## 1. Vault (`crypto.Service` + `MasterKeyProvider`)

| STRIDE | Threat | Mitigation | Residual |
|---|---|---|---|
| S | Attacker forges KMS decrypt request | Provider uses IAM role + audited KMS logs | Low |
| T | DEK or ciphertext tampered in DB | AES-GCM auth tag rejects tampered input | Low |
| R | Key rotation without audit trail | `keyrotation_service` writes audit entries | Low |
| I | Master key exfiltrated via logs | Master key never logged; only cached in RAM | Low |
| D | KMS throttle stalls startup | MasterKeyProvider retries with backoff | Medium |
| E | Compromised process reads RAM | Linux namespaces + minimal container image | Medium |

**Follow-ups:** secure-zero master key in memory on shutdown; add hardware
attestation for nodes handling master key.

## 2. OAuth IdP (`oauth_service.go`)

| STRIDE | Threat | Mitigation | Residual |
|---|---|---|---|
| S | Authorization code replay | Single-use codes, 60s TTL, bound to `client_id` | Low |
| T | Token introspection tampering | HMAC-signed access tokens | Low |
| R | Client registration without audit | All OAuth client ops written to audit log | Low |
| I | Refresh token leak via logs | Tokens hashed before storage; logs redact | Low |
| D | Credential stuffing on `/authorize` | Per-IP rate limit + exponential backoff | Medium |
| E | Scope escalation via token reuse | Tokens scoped per-client; PKCE required | Low |

**Follow-ups:** DPoP for public clients; mTLS for confidential clients.

## 3. MCP Gateway

| STRIDE | Threat | Mitigation | Residual |
|---|---|---|---|
| S | Malicious MCP server impersonates legit | Registry binds server to owner + signature | Medium |
| T | Tool response tampered in transit | TLS to registered server; response schema check | Low |
| I | Secret injected into wrong tool | Injection scoped per project + environment | Low |
| D | Slow MCP server stalls gateway | Per-call 10s timeout + circuit breaker | Low |
| E | Tool call bypasses auth | Gateway enforces JWT/API key before routing | Low |

**Follow-ups:** require signed manifests for community MCP servers.

## 4. Promotion Engine

| STRIDE | Threat | Mitigation | Residual |
|---|---|---|---|
| T | Secret modified between diff and apply | Transactional apply; diff re-validated | Low |
| R | Approver identity spoofed | Approver must re-auth; audit captures JWT `sub` | Low |
| I | Diff leaks secret plaintext | Diff redacts values; only keys + action shown | Low |
| D | Flapping promotion retries saturate DB | Per-project promotion rate limit | Low |
| E | Non-approver promotes to PROD | PROD requires explicit `promote` scope | Low |

**Follow-ups:** N-of-M approvals for PROD (tracked in Phase 12 policies).

---

## Assumptions

- Kubernetes or equivalent container runtime with network policies
- Postgres with `sslmode=require`, offline encrypted backups
- Master key managed by KMS in production (Env provider is dev-only)
- Operators keep host OS and container images patched
