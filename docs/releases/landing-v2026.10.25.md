# KeepSave landing-v2026.10.25

Release date: 25 October 2026. Scope: public static landing and the accompanying
frontend/documentation integration. The application version remains the
unreleased `1.4.0-rc.1` candidate. This tag does not qualify backend services,
provider sign-in, SMTP, harnesses, runner isolation or production vault operation.

## Changes

- Six-part story: use cases, approach, features, clearly labeled fictional
  feedback, FAQ and contact.
- Original Gargantua artwork, warm charcoal/gold and ivory/bronze tokens,
  refined Field Twist assets and favicon.
- Shared application/auth/widget theme; 13 searchable developer task guides.
- Corrected UAT selections, masks and repeated planet gradients.
- Sidebar without horizontal overflow or visible scrollbars; wheel, keyboard
  focus and native forced-colors scrollbars remain available.

## Delivery and acceptance

The owner requested develop integration, Vercel publication and a release tag.
No `main` merge is included. The Vercel target is the existing
`santapongs-projects/keepsave-landing` project, serving
[keepsave.draveniq.dev](https://keepsave.draveniq.dev/). Only ten allowlisted
public files are deployed. Login, API, MCP and application routes are excluded;
no provider credentials, private source or database is uploaded.

The dated [acceptance record](../validation/2026-10-25-documentation-ux/README.md)
retains the 172-test frontend baseline and final seven focused tests, lint,
application/static packaging and 13/13 actual Robot checks. The verified
675-file transferred snapshot matched the release source before documentation
and ignore-rule edits. All ten final static hashes match the tested package.
These are local/Pi synthetic results, not a green remote pipeline or real-provider
acceptance. Earlier failures and their fixes remain recorded.

Publication status and deployment identity will be recorded after staging and
production checks. The previous production deployment is retained as the static
rollback target. No database migration, key change or application cutover occurs.

## Artifact-specific risk assessment

The public output contains HTML/CSS/JavaScript, brand SVGs and one WebP only.
It has no server runtime, Vite/Vitest endpoint, source maps, API transport or
credential fields. CSP denies connections and form submission; outbound setup
and contact links are ordinary navigation. Mock quotes remain visibly labeled.

A fresh lockfile-only npm audit on 25 October reported two high and two moderate
package-level findings: Vite, source-map-js, Vitest and @vitest/mocker. The Vite
editor/dev middleware, indexed source-map processing and Vitest mock endpoints
are build/test features excluded from this static deployment. No dependency or
severity threshold was changed. Full application/build-tool remediation remains
open; this is not a clean audit or a waiver of application CI. Source inventory
and static exclusion support publication of this bounded package, not a general
security certification. Audit JSON is retained with the local release evidence.

The older full-repository image-scan failure, independent Security/Tech Lead
reviews and provider/recovery/operational gates remain unresolved. The application
and backend must not be advertised as production-released by this tag.

## Operator verification and rollback

Stage the production-target static build without domain assignment, verify exact
file hashes, security headers, fictional-feedback labels and denied private
routes, then promote that deployment. Recheck the custom domain and browser.
Rollback means promoting the previously recorded static deployment; it does not
change application data. Release evidence is retained locally in
`/mnt/data/keepsave-release-2026-10-25/` with source, tag, package and deployment
identities. Do not upload private local test contexts or operator environment files.
