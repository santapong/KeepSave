# -*- coding: utf-8 -*-
"""Generates KeepSave's architecture diagrams as static SVG.

Deliberately a light 'paper' canvas with explicit colours: GitHub sanitises
SVG and does not reliably honour prefers-color-scheme inside <img>, so a
deterministic palette is the only way one asset reads correctly in both
GitHub themes.
"""
import os, html

OUT = "docs/diagrams"
FONT = "Geist, Inter, ui-sans-serif, system-ui, sans-serif"
MONO = "Geist Mono, ui-monospace, SFMono-Regular, Menlo, monospace"

PAPER   = "#fbfaff"
INK     = "#1e1b2e"
MUTED   = "#5b5772"
LINE    = "#8b87a3"
ACCENT  = "#7c3aed"       # KeepSave periwinkle violet
ACCENT_D= "#5b21b6"
TEAL    = "#0f9488"
EXT     = "#9490a8"

STYLES = {
    # fill, stroke, title colour, subtitle colour
    "person":    ("#6d28d9", ACCENT_D, "#ffffff", "#ddd6fe"),
    "system":    (ACCENT,    ACCENT_D, "#ffffff", "#e9d5ff"),
    "external":  (EXT,       "#6b6880", "#ffffff", "#e5e4ea"),
    "container": ("#ffffff", ACCENT,   INK,       MUTED),
    "store":     ("#f5f3ff", "#a78bfa", INK,      MUTED),
    "component": ("#f8f7ff", "#c4b5fd", INK,      MUTED),
    "node":      ("#ffffff", LINE,      INK,      MUTED),
    "accentbox": ("#ede9fe", ACCENT,    ACCENT_D, MUTED),
}

def esc(s): return html.escape(s, quote=True)

def header(w, h, title):
    return (f'<svg xmlns="http://www.w3.org/2000/svg" width="{w}" height="{h}" '
            f'viewBox="0 0 {w} {h}" role="img" aria-label="{esc(title)}">\n'
            f'<title>{esc(title)}</title>\n'
            f'<defs><marker id="a" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" '
            f'markerHeight="7" orient="auto-start-reverse">'
            f'<path d="M0 0 L10 5 L0 10 z" fill="{LINE}"/></marker></defs>\n'
            f'<rect width="{w}" height="{h}" fill="{PAPER}"/>\n')

def caption(x, y, text, size=13, color=MUTED, anchor="start", weight="500", font=FONT):
    return (f'<text x="{x}" y="{y}" font-family="{font}" font-size="{size}" '
            f'font-weight="{weight}" fill="{color}" text-anchor="{anchor}">{esc(text)}</text>\n')

def box(x, y, w, h, title, subtitle="", tech="", kind="container", r=8):
    """SVG <text> does not wrap, so any "\n" in a label is expanded into its
    own line here — otherwise long subtitles silently overflow the box."""
    fill, stroke, tc, sc = STYLES[kind]
    s = f'<rect x="{x}" y="{y}" width="{w}" height="{h}" rx="{r}" fill="{fill}" stroke="{stroke}" stroke-width="1.5"/>\n'
    rows = []                                    # (text, size, colour, weight, font, line-height)
    for ln in str(title).split("\n"):
        rows.append((ln, 13.5, tc, "600", FONT, 17))
    if subtitle:
        for ln in str(subtitle).split("\n"):
            rows.append((ln, 11, sc, "400", FONT, 14))
    if tech:
        for ln in str(tech).split("\n"):
            rows.append((ln, 10, sc, "400", MONO, 13))
    total = sum(rr[5] for rr in rows)
    cy = y + h/2 - total/2 + 12
    for text, size, col, wt, fnt, lh in rows:
        s += caption(x + w/2, cy, text, size, col, "middle", wt, fnt)
        cy += lh
    return s

def arrow(x1, y1, x2, y2, label="", dash=False, side="mid", off=0):
    d = ' stroke-dasharray="5 4"' if dash else ''
    s = (f'<path d="M{x1} {y1} L{x2} {y2}" fill="none" stroke="{LINE}" '
         f'stroke-width="1.6"{d} marker-end="url(#a)"/>\n')
    if label:
        mx, my = (x1+x2)/2, (y1+y2)/2
        tw = len(label) * 5.6 + 10
        s += (f'<rect x="{mx-tw/2}" y="{my-9+off}" width="{tw}" height="16" rx="4" '
              f'fill="{PAPER}" opacity="0.95"/>\n')
        s += caption(mx, my+3+off, label, 10.5, MUTED, "middle", "500")
    return s

def legend(x, y, items):
    s = ""
    for i, (kind, lbl) in enumerate(items):
        fill, stroke, _, _ = STYLES[kind]
        yy = y + i*20
        s += f'<rect x="{x}" y="{yy}" width="16" height="12" rx="3" fill="{fill}" stroke="{stroke}" stroke-width="1.2"/>\n'
        s += caption(x+23, yy+10.5, lbl, 11, MUTED)
    return s

def write(name, body):
    path = os.path.join(OUT, name)
    with open(path, "w", encoding="utf-8") as f:
        f.write(body + "</svg>\n")
    print(f"  {path}  ({os.path.getsize(path)} bytes)")

# ------------------------------------------------------------------ #
# C4 L1 — System Context
# ------------------------------------------------------------------ #
w, h = 900, 560
s = header(w, h, "KeepSave — C4 Level 1: System Context")
s += caption(28, 34, "C4 · Level 1 — System Context", 15, INK, weight="600")
s += caption(28, 54, "Who and what KeepSave talks to. One box for KeepSave; everything else is outside it.", 12)
s += box(60, 96, 190, 74, "Developer", "Stores and promotes secrets", kind="person", r=10)
s += box(350, 96, 200, 74, "AI Agent", "Fetches scoped secrets", "Claude / MCP client", kind="person", r=10)
s += box(650, 96, 190, 74, "CI/CD Pipeline", "Injects env at deploy", "Actions / GitLab", kind="person", r=10)
s += box(300, 268, 300, 96, "KeepSave", "Encrypted vault, OAuth 2.0 provider\nand MCP server hub", "Go + React", kind="system", r=10)
s += box(40, 452, 190, 72, "MCP Servers", "Tools invoked via gateway", kind="external")
s += box(268, 452, 190, 72, "GitHub", "MCP server registry source", kind="external")
s += box(496, 452, 170, 72, "KMS", "Holds the master key", kind="external")
s += box(700, 452, 160, 72, "SMTP / OIDC", "Notify, federate", kind="external")
s += arrow(155, 170, 380, 266, "manages projects, promotes")
s += arrow(450, 170, 450, 266, "reads secrets via API key")
s += arrow(745, 170, 520, 266, "pulls env at build")
s += arrow(380, 364, 150, 450, "routes tool calls")
s += arrow(420, 364, 355, 450, "imports servers", dash=True)
s += arrow(490, 364, 570, 450, "unwraps DEKs")
s += arrow(540, 364, 760, 450, "sends mail / SSO", dash=True)
s += legend(60, 210, [("person","Actor"),("system","KeepSave (in scope)"),("external","External system")])
write("c4-1-context.svg", s)

# ------------------------------------------------------------------ #
# C4 L2 — Container
# ------------------------------------------------------------------ #
w, h = 960, 640
s = header(w, h, "KeepSave — C4 Level 2: Containers")
s += caption(28, 34, "C4 · Level 2 — Containers", 15, INK, weight="600")
s += caption(28, 54, "The deployable/runnable pieces inside KeepSave and how they communicate.", 12)
s += f'<rect x="40" y="76" width="880" height="470" rx="12" fill="none" stroke="{ACCENT}" stroke-width="1.4" stroke-dasharray="7 5"/>\n'
s += caption(56, 96, "KeepSave system boundary", 11, ACCENT_D, weight="600")
s += box(70, 118, 200, 78, "Dashboard SPA", "Projects, secrets, promotion", "React 19 + Vite", kind="container")
s += box(310, 118, 200, 78, "Embeddable Widget", "<keepsave-widget>", "Web Component", kind="container")
s += box(550, 118, 200, 78, "SDKs + CI", "Go · Node · Python", "Terraform, Actions", kind="container")
s += box(240, 258, 220, 84, "REST API", "Auth, projects, secrets,\npromotion, OAuth", "Go + Gin", kind="container")
s += box(530, 258, 220, 84, "MCP Gateway", "Routes tool calls,\ninjects secrets as env", "JSON-RPC 2.0", kind="container")
s += box(240, 400, 220, 76, "Crypto Layer", "AES-256-GCM envelope,\nper-project DEKs", kind="accentbox")
s += box(530, 400, 220, 76, "Promotion Engine", "Diff, approve, apply,\nrollback, audit", kind="accentbox")
s += box(300, 566, 180, 58, "Database", "PostgreSQL / MySQL / SQLite", kind="store")
s += box(520, 566, 180, 58, "Audit + Gateway Log", "Append-only event trail", kind="store")
s += arrow(170, 196, 300, 256, "HTTPS / JSON")
s += arrow(410, 196, 380, 256, "HTTPS")
s += arrow(650, 196, 640, 256, "API key")
s += arrow(460, 300, 528, 300, "resolves secrets")
s += arrow(330, 342, 330, 398, "seal / unseal")
s += arrow(640, 342, 640, 398, "invokes")
s += arrow(350, 476, 380, 564, "reads / writes")
s += arrow(620, 476, 600, 564, "appends")
write("c4-2-container.svg", s)

# ------------------------------------------------------------------ #
# C4 L3 — Component (backend)
# ------------------------------------------------------------------ #
w, h = 960, 600
s = header(w, h, "KeepSave — C4 Level 3: REST API components")
s += caption(28, 34, "C4 · Level 3 — Components inside the REST API", 15, INK, weight="600")
s += caption(28, 54, "Handler -> service -> repository. The crypto layer is reached only through services.", 12)
s += f'<rect x="40" y="76" width="880" height="440" rx="12" fill="none" stroke="{ACCENT}" stroke-width="1.4" stroke-dasharray="7 5"/>\n'
s += caption(56, 96, "REST API container", 11, ACCENT_D, weight="600")
cols = ["Auth", "Projects", "Secrets", "Promotion", "OAuth", "MCP Hub"]
for i, c in enumerate(cols):
    s += box(64 + i*143, 116, 128, 52, c, "handler", kind="component", r=6)
svc = [("AuthService","JWT + API keys"), ("SecretService","seal / reveal"), ("PromotionService","diff + approve"),
       ("OAuthService","4 grant flows"), ("MCPService","registry + proxy"), ("AuditService","event taxonomy")]
for i, (n, d) in enumerate(svc):
    x = 64 + (i % 3) * 288
    y = 210 + (i // 3) * 84
    s += box(x, y, 264, 66, n, d, kind="component", r=6)
s += box(64, 400, 400, 62, "Crypto Layer", "AES-256-GCM · per-project DEK · master key from KMS", kind="accentbox", r=6)
s += box(496, 400, 360, 62, "Repository Layer", "SQL, driver-agnostic", kind="store", r=6)
s += arrow(480, 168, 480, 206, "dispatch")
s += arrow(260, 342, 260, 398, "seal / unseal")
s += arrow(660, 342, 660, 398, "persist")
s += arrow(464, 431, 494, 431, "ciphertext only")
s += caption(64, 496, "Invariant: no handler touches the crypto layer or returns a plaintext secret value directly.", 11.5, ACCENT_D, weight="600")
s += caption(64, 540, "Every state-mutating handler emits an audit event from the canonical taxonomy (docs/AUDIT_LOG_COVERAGE.md).", 11.5)
write("c4-3-component-api.svg", s)

# ------------------------------------------------------------------ #
# 4+1 — Logical view
# ------------------------------------------------------------------ #
w, h = 900, 470
s = header(w, h, "KeepSave — 4+1 Logical view")
s += caption(28, 34, "4+1 · Logical view — the domain model", 15, INK, weight="600")
s += caption(28, 54, "What the system is made of, from the end user's point of view.", 12)
ent = [("Organisation", "tenants, SSO", 60, 92), ("Project", "unit of isolation", 300, 92), ("Environment", "alpha · uat · prod", 540, 92),
       ("Secret", "name + sealed value", 780, 92)]
for n, d, x, y in ent[:3]:
    s += box(x, y, 200, 68, n, d, kind="container")
s += box(700, 92, 150, 68, "Secret", "sealed value", kind="container")
s += box(60, 226, 200, 68, "API Key", "scoped to project/env", kind="container")
s += box(300, 226, 200, 68, "OAuth Client", "4 grant flows", kind="container")
s += box(540, 226, 200, 68, "MCP Server", "registered from GitHub", kind="container")
s += box(60, 356, 200, 66, "Promotion", "diff · approve · apply", kind="accentbox")
s += box(300, 356, 200, 66, "Lease", "time-boxed access", kind="accentbox")
s += box(540, 356, 310, 66, "Audit Event", "append-only, every mutation", kind="store")
s += arrow(260, 126, 298, 126, "1..n")
s += arrow(500, 126, 538, 126, "1..n")
s += arrow(740, 126, 698, 126, "1..n")
s += arrow(160, 160, 160, 224, "issues")
s += arrow(400, 160, 400, 224, "owns")
s += arrow(640, 160, 640, 224, "consumes")
s += arrow(160, 294, 160, 354, "target of")
s += arrow(400, 294, 400, 354, "grants")
s += arrow(640, 294, 660, 354, "records")
write("view-logical.svg", s)

# ------------------------------------------------------------------ #
# 4+1 — Process view
# ------------------------------------------------------------------ #
w, h = 940, 420
s = header(w, h, "KeepSave — 4+1 Process view")
s += caption(28, 34, "4+1 · Process view — concurrency and runtime flows", 15, INK, weight="600")
s += caption(28, 54, "Three independent request paths share one crypto layer and one audit trail.", 12)
s += box(50, 96, 190, 62, "HTTP request loop", "Gin handlers, per-request goroutine", kind="container")
s += box(50, 186, 190, 62, "MCP gateway proxy", "per tool call", kind="container")
s += box(50, 276, 190, 62, "Background workers", "lease expiry, backups", kind="container")
s += box(370, 96, 200, 62, "Service layer", "business rules", kind="component")
s += box(370, 186, 200, 62, "Process runner", "spawns MCP server", kind="component")
s += box(370, 276, 200, 62, "Scheduler", "ticks, retries", kind="component")
s += box(690, 140, 200, 70, "Crypto layer", "serialised DEK unwrap", kind="accentbox")
s += box(690, 250, 200, 70, "Audit sink", "append-only writes", kind="store")
for y in (127, 217, 307):
    s += arrow(240, y, 368, y)
s += arrow(570, 127, 688, 165, "unseal")
s += arrow(570, 217, 688, 190, "inject env")
s += arrow(570, 307, 688, 285, "emit")
s += arrow(790, 210, 790, 248, "every mutation")
s += caption(50, 380, "Rate limiting keys on the derived client IP; TRUSTED_PROXIES decides whether X-Forwarded-For is believed.", 11.5)
write("view-process.svg", s)

# ------------------------------------------------------------------ #
# 4+1 — Development view
# ------------------------------------------------------------------ #
w, h = 900, 460
s = header(w, h, "KeepSave — 4+1 Development view")
s += caption(28, 34, "4+1 · Development view — how the repository is laid out", 15, INK, weight="600")
s += caption(28, 54, "Module boundaries as they appear on disk.", 12)
s += box(50, 92, 250, 250, "backend/", "", "Go 1.24 + Gin", kind="node")
for i, (n, d) in enumerate([("cmd/server", "entry point"), ("internal/api", "handlers, middleware"),
                             ("internal/service", "business logic"), ("internal/crypto", "AES-256-GCM"),
                             ("internal/repository", "SQL access")]):
    s += box(66, 130 + i*40, 218, 32, n, d, kind="component", r=5)
s += box(330, 92, 250, 210, "frontend/", "", "React 19 + Vite", kind="node")
for i, (n, d) in enumerate([("src/pages", "route screens"), ("src/components/cosmic", "Event Horizon"),
                             ("src/styles", "cosmic + landing css"), ("src/embed", "widget SDK")]):
    s += box(346, 130 + i*40, 218, 32, n, d, kind="component", r=5)
s += box(610, 92, 240, 130, "sdks/", "", "", kind="node")
for i, n in enumerate(["go", "nodejs", "python"]):
    s += box(626, 130 + i*28, 208, 22, n, "", kind="component", r=5)
s += box(610, 240, 240, 102, "integrations/", "", "", kind="node")
for i, n in enumerate(["github-action", "gitlab-ci", "terraform"]):
    s += box(626, 264 + i*26, 208, 22, n, "", kind="component", r=5)
s += box(50, 366, 800, 56, "docs/  ·  tests/  ·  helm/  ·  .github/workflows", "ADRs, threat model, runbook, pyramid, CI", kind="store")
s += caption(50, 356, "Security Engineer holds veto on internal/crypto, internal/auth and the promotion engine.", 11.5, ACCENT_D, weight="600")
write("view-development.svg", s)

# ------------------------------------------------------------------ #
# 4+1 — Physical view
# ------------------------------------------------------------------ #
w, h = 900, 480
s = header(w, h, "KeepSave — 4+1 Physical view")
s += caption(28, 34, "4+1 · Physical view — deployment topology", 15, INK, weight="600")
s += caption(28, 54, "Split-origin deploy: static frontend and API are separate Vercel projects.", 12)
s += box(50, 92, 380, 150, "Vercel — keep-save", "", "static SPA + CDN", kind="node", r=10)
s += box(70, 138, 340, 44, "dist/ bundle", "CSP: script-src 'self'; fonts from gstatic", kind="component", r=6)
s += box(70, 190, 340, 40, "/assets/* immutable, index.html no-cache", "", kind="component", r=6)
s += box(470, 92, 380, 150, "Vercel — keep-save-api", "", "Go binary", kind="node", r=10)
s += box(490, 138, 340, 44, "REST API + MCP gateway", "migrations embedded in binary", kind="component", r=6)
s += box(490, 190, 340, 40, "Prometheus metrics · OpenTelemetry", "", kind="component", r=6)
s += box(180, 300, 240, 76, "Database", "PostgreSQL 16 (managed)", kind="store", r=10)
s += box(490, 300, 200, 76, "KMS", "master key, never in DB", kind="accentbox", r=10)
s += box(720, 300, 130, 76, "MCP servers", "process runner", kind="node", r=10)
s += arrow(430, 160, 468, 160, "HTTPS  connect-src")
s += arrow(300, 242, 300, 298, "TLS")
s += arrow(600, 242, 590, 298, "unwrap DEK")
s += arrow(760, 242, 780, 298, "spawn / JSON-RPC")
s += caption(50, 430, "Also shipped: docker-compose.yml for local/self-host, and helm/ for Kubernetes.", 11.5)
s += caption(50, 452, "Deploys are gated by CI: lint, tests, npm audit, gosec, govulncheck, CodeQL.", 11.5)
write("view-physical.svg", s)

# ------------------------------------------------------------------ #
# 4+1 — Scenario (+1)
# ------------------------------------------------------------------ #
w, h = 940, 520
s = header(w, h, "KeepSave — 4+1 Scenario: agent tool call with secret injection")
s += caption(28, 34, "4+1 · Scenarios (+1) — an agent calls an MCP tool", 15, INK, weight="600")
s += caption(28, 54, "The scenario that ties the other four views together. The agent never receives the credential.", 12)
actors = [("AI Agent", 60), ("MCP Gateway", 270), ("Secret Vault", 500), ("MCP Server", 730)]
for n, x in actors:
    s += box(x, 92, 150, 46, n, "", kind="container", r=8)
    s += f'<path d="M{x+75} 138 L{x+75} 452" stroke="{LINE}" stroke-width="1.2" stroke-dasharray="4 5"/>\n'
steps = [
    (135, 345, 178, "1  tools/call (name, args)"),
    (345, 575, 226, "2  resolve env_mappings"),
    (575, 345, 274, "3  decrypted values (in memory)"),
    (345, 805, 322, "4  exec with secrets as env vars"),
    (805, 345, 370, "5  tool result"),
    (345, 135, 418, "6  result to agent"),
]
for x1, x2, y, label in steps:
    s += arrow(x1, y, x2, y, label, off=-4)
s += f'<rect x="60" y="452" width="820" height="44" rx="8" fill="#ede9fe" stroke="{ACCENT}" stroke-width="1.4"/>\n'
s += caption(76, 479, "Audit: mcp.tool.called is written before the result returns. The plaintext never leaves the gateway process.", 11.5, ACCENT_D, weight="600")
write("view-scenario-mcp.svg", s)

print("done")

# ------------------------------------------------------------------ #
# Flow — OAuth 2.0 authorization code
# ------------------------------------------------------------------ #
w, h = 940, 430
s = header(w, h, "KeepSave — OAuth 2.0 authorization code flow")
s += caption(28, 34, "Flow — OAuth 2.0 authorization code", 15, INK, weight="600")
s += caption(28, 54, "KeepSave acts as the identity provider. PKCE, client credentials and refresh token are also supported.", 12)
lanes = [("Client App", 60), ("KeepSave OAuth IdP", 350), ("Resource Server", 690)]
for n, x in lanes:
    s += box(x, 88, 190, 46, n, "", kind="container", r=8)
    s += f'<path d="M{x+95} 134 L{x+95} 388" stroke="{LINE}" stroke-width="1.2" stroke-dasharray="4 5"/>\n'
steps = [
    (155, 445, 176, "1  GET /oauth/authorize  (client_id, redirect_uri, scope)"),
    (445, 155, 218, "2  authorization code"),
    (155, 445, 260, "3  POST /oauth/token  (code, client_id, client_secret)"),
    (445, 155, 302, "4  access token + refresh token"),
    (155, 785, 344, "5  API request  (Bearer token)"),
    (785, 155, 386, "6  response"),
]
for x1, x2, y, label in steps:
    s += arrow(x1, y, x2, y, label, off=-4)
write("flow-oauth.svg", s)

# ------------------------------------------------------------------ #
# Flow — environment promotion pipeline
# ------------------------------------------------------------------ #
w, h = 940, 400
s = header(w, h, "KeepSave — environment promotion pipeline")
s += caption(28, 34, "Flow — environment promotion", 15, INK, weight="600")
s += caption(28, 54, "Alpha to UAT to PROD. Every hop is diff-reviewed, audit-logged and reversible.", 12)
stages = [("Alpha", 70), ("UAT", 390), ("PROD", 710)]
for n, x in stages:
    kind = "accentbox" if n == "PROD" else "container"
    s += box(x, 100, 160, 62, n, "environment scope", kind=kind, r=10)
s += arrow(230, 131, 388, 131, "promote")
s += arrow(550, 131, 708, 131, "promote")
notes = [
    (70,  "Diff preview\nInstant apply\nAudit logged"),
    (390, "Diff preview\nInstant apply\nAudit logged"),
    (710, "Multi-party approval\nAudit logged\nRollback supported"),
]
for x, txt in notes:
    s += box(x, 196, 160, 86, txt.split("\n")[0], "\n".join(txt.split("\n")[1:]), kind="component", r=8)
    s += arrow(x+80, 162, x+80, 194)
s += f'<rect x="70" y="312" width="800" height="46" rx="8" fill="#ede9fe" stroke="{ACCENT}" stroke-width="1.4"/>\n'
s += caption(86, 340, "Kill switch: KEEPSAVE_PROMOTIONS_ENABLED=false makes /promote and /approve return 503 (docs/RUNBOOK.md section 8).", 11.5, ACCENT_D, weight="600")
write("flow-promotion.svg", s)
