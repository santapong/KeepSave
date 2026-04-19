# KeepSave Incident Runbook

**Audience:** on-call operators. **Severity taxonomy:** P0 = data loss or
production outage; P1 = security incident with contained blast radius;
P2 = degraded service.

## Contents

1. Lost master key (P0)
2. Compromised API key (P1)
3. Database failover (P0)
4. Rotate-everything drill (P2)
5. govulncheck regression (P2)

---

## 1. Lost master key

**Symptom:** startup fails with `MasterKeyProvider returned zero bytes` or
all decrypt operations return `message authentication failed`.

**Containment**

1. Put service into read-only mode (`ENABLE_WRITES=false`).
2. Rotate the KMS key alias to the last known good version (AWS KMS:
   `aws kms update-alias --alias-name alias/keepsave-master --target-key-id ...`).
3. Restart one pod, confirm DEK unwrap succeeds against a sample project.

**Recovery**

1. If the key is truly lost, restore from the most recent encrypted backup
   (see section 4) which carries its own DEK wrapped by the old master.
2. If backups are also unreadable, re-provision secrets from source of truth.
3. Post-mortem required. Add a timeline entry to `docs/INCIDENTS.md`.

**Prevent recurrence:** enable KMS key deletion protection; add a weekly
`RestoreSnapshot` smoke test to CI.

## 2. Compromised API key

**Symptom:** anomaly service raises `UnusualKeyAccess` or a user reports
leak.

**Containment** (target: under 5 minutes)

1. Revoke the key: `keepsave api-key revoke <key-id>`.
2. Invalidate dependent leases: `keepsave lease revoke --api-key <key-id>`.
3. Rotate any secrets the key could read: per-project `POST /rotate-keys`.

**Recovery**

1. Export audit log for the key's active window.
2. Identify accessed secrets; treat them as compromised.
3. Notify affected owners via configured webhooks.

## 3. Database failover

**Symptom:** `/readyz` returns 503; backend logs show `driver: bad connection`.

**Steps**

1. Confirm primary is down (cloud console or `pg_isready`).
2. Promote replica to primary.
3. Update `DATABASE_URL` secret in Kubernetes and `kubectl rollout restart`
   the backend deployment.
4. Verify migrations are at expected version: `SELECT * FROM migrations ORDER BY id DESC LIMIT 1`.
5. Resume traffic.

## 4. Rotate-everything drill

Quarterly exercise to prove rotation plumbing works.

1. Rotate master key: issue new KMS data key, update `KEEPSAVE_KMS_CIPHERTEXT`.
2. Roll pods one at a time; confirm each accepts the new ciphertext.
3. For each project: `POST /api/v1/projects/:id/rotate-keys`.
4. Rotate all API keys older than 90 days.
5. Rotate JWT signing secret (forces all users to re-auth).
6. Create fresh backup snapshot; verify restore on staging.

## 5. govulncheck regression

**Symptom:** CI `security-scan` job fails on a previously green branch.

1. Review the report artifact (90-day retention).
2. If the vuln is **called**: pin or upgrade the dependency in the same PR.
3. If **not called**: annotate `errorlog.md` and open a Dependabot PR to
   upgrade on the next cycle. Do not merge unless the team agrees.

## Contact paths

- Primary on-call: PagerDuty `keepsave-oncall`
- Security escalation: `security@<org>` + post in `#sec-incident`
- Leadership notification for P0 after 30 minutes
