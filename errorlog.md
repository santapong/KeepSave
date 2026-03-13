# Error Log

## Security Scan - govulncheck

### 2026-03-13: govulncheck type resolution failure (RESOLVED)

**Error:**
```
go install golang.org/x/vuln/cmd/govulncheck@latest
2026/03/13 09:12:56 internal error: package "golang.org/x/sys/unix" without types was imported from "github.com/mattn/go-isatty"
Error: Process completed with exit code 1.
```

**Root Cause:**
Two-layer issue:
1. **CI Go version mismatch (primary):** CI workflow used `go-version: '1.22'` but `go.mod` declared `go 1.24.0`. govulncheck v1.1.4's package loader (`x/tools/go/packages`) cannot properly resolve types for platform-constrained packages like `x/sys/unix` under this mismatch. Tracked as [golang/go#77401](https://github.com/golang/go/issues/77401).
2. **Stale transitive dependencies (contributing):** `bytedance/sonic v1.9.1` used the deprecated `chenzhuoyu/base64x` with assembly build-tag patterns incompatible with Go 1.24's stricter package loading.

**Fix Applied:**
- Upgraded CI `go-version` from `'1.22'` to `'1.25.8'`
- Upgraded `bytedance/sonic` v1.9.1 -> v1.11.6 (replaces `chenzhuoyu/base64x` with `cloudwego/base64x` + `cloudwego/iasm`)
- Upgraded `klauspost/cpuid/v2` v2.2.4 -> v2.2.7
- Upgraded `golang.org/x/arch` v0.3.0 -> v0.8.0
- Upgraded `go-isatty` v0.0.19 -> v0.0.20
- Updated Dockerfile from `golang:1.22-alpine` to `golang:1.25-alpine`

**Commits:** `9b7dba9`, `2f3a4d5`, `4eb2435`, `e7874e5`

---

### 2026-03-13: Go stdlib vulnerabilities GO-2026-4602, GO-2026-4601 (RESOLVED)

**Error:**
```
Vulnerability #1: GO-2026-4602
    FileInfo can escape from a Root in os
    Found in: os@go1.25.7
    Fixed in: os@go1.25.8

Vulnerability #2: GO-2026-4601
    Incorrect parsing of IPv6 host literals in net/url
    Found in: net/url@go1.25.7
    Fixed in: net/url@go1.25.8
```

**Root Cause:**
Go standard library vulnerabilities in `os` and `net/url` packages. CI resolved `go-version: '1.25'` to Go 1.25.7, but both fixes require Go 1.25.8.

**Affected Code Paths:**
- `internal/repository/migrate.go:23` -> `os.ReadDir`
- `internal/service/webhook_service.go:158` -> `http.Client.Do` -> `url.Parse`
- `cmd/server/main.go:109` -> `gin.Engine.Run` -> `url.ParseRequestURI`

**Fix Applied:**
- Pinned CI `go-version` to `'1.25.8'` (exact patch version with security fixes)
- Added govulncheck report artifact upload (90-day retention) for audit trail

**Commit:** `e7874e5`, `0280ac7`

---

### Additional: 3 non-called dependency vulnerabilities (NOTED)

govulncheck also found 3 vulnerabilities in imported packages and 4 in required modules where the vulnerable code paths are **not called** by KeepSave. These do not affect the application but should be reviewed during the next dependency upgrade cycle. Run `govulncheck -show verbose ./...` for details.
