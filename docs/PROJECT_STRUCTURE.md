# KeepSave project structure

The canonical project is **`/mnt/data/company/apps/KeepSave`**, as selected by the
owner on 4 October 2026. Run project commands there. It is the permanent `develop`
checkout; `main` remains the release/source-publication branch in the same Git
repository. Supporting worktrees share Git history and are not separate products.

## Local workspace arrangement

```text
/mnt/data/company/
├── apps/
│   └── KeepSave/                         canonical repository / develop
└── .worktrees/
    └── KeepSave/
        ├── backend-platform/             temporary implementation/review checkout
        ├── landing/                      preserved landing branch and local changes
        └── README.md                     local folder roles / resume pointer
```

The earlier `apps/KeepSave-backend-platform` and `apps/KeepSave-landing` paths are
historical locators. Use `git worktree list` to discover live paths and check
`git status` in each before integration. Worktrees are moved with Git so their
metadata and dirty files remain intact. Do not copy, reset or delete a dirty
worktree to tidy folders. Private run evidence and recovery material remain
outside the repository; they are not product source or publishable assets.

## Repository ownership

```text
KeepSave/
├── README.md / CHANGELOG.md / Roadmap.md   product, changes and gated delivery
├── CLAUDE.md                              development/review rules
├── backend/
│   ├── cmd/                               API, worker, runner and operator tools
│   ├── internal/                          shared core and explicit adapters
│   └── migrations/                        additive PostgreSQL/legacy dialect SQL
├── frontend/
│   ├── src/                               application and widget source
│   └── public/                            accepted Field Twist assets
├── deploy/
│   ├── self-hosted/                        control-host installation reference
│   └── runner/                            separate-host execution reference
├── sdks/                                  existing Go / Node / Python clients
├── integrations/                          Actions / GitLab / Terraform adapters
├── scripts/                               fixtures, type generation and diagrams
├── tests/                                 client/Robot/negative-test definitions
├── docs/
│   ├── README.md                          current documentation entry point
│   ├── system/                            current subsystem descriptions
│   ├── adr/                               decisions and review status
│   ├── design/                            dated plans and implementation records
│   ├── validation/                        dated executed evidence
│   ├── releases/                          candidate/publication notes
│   ├── diagrams/ / assets/                 branded source and diagram artifacts
│   ├── audits/ / research/                 historical findings and research
│   └── archive/                           earlier root notes/audits/phase summaries
└── .github/workflows/                      integration verification
```

Older Helm/cloud deployment files and legacy service sources remain for
compatibility/history. Their presence does not make them current accepted
installation profiles. [Architecture](ARCHITECTURE.md) assigns domain ownership;
[system chapters](system/README.md) describe the concrete packages. Do not create a
module per harness: protocol and native packaging adapters share the same policy,
run, broker and credential-custody services.

## Where new work belongs

Start a short-lived branch from freshly reconciled `develop`. Keep source changes
inside their owning module, contract changes in the maintained OpenAPI file,
operator instructions in current guides, decisions in `adr/`, and dated execution
receipts in `validation/`. Preserve historical notes instead of overwriting their
results. [The documentation map](DOCUMENTATION_MAP.md) classifies every tracked
Markdown document and identifies the current entry points.
