# Event Horizon — landing story

> **Design lineage, clarified 2026-10-04.** The accepted Event Horizon narrative remains the landing direction. This record preserves the September 28 implementation and original-mark references; the subsequently selected Field Twist identity and current assets are maintained in the [brand guide](../../BRANDING.md). Local preview and publication statements below describe their original dated requests, not the current deployment status.

Date: 2026-09-28. Owner selected **Event Horizon** from the three visual directions and asked for storytelling that makes KeepSave easy to understand. This authorizes local landing implementation; it does not authorize publication. Exact selected image: `selected-reference.png`, SHA-256 `29208a53ccc8e2c4cb12e9b5fc9bcfc08b59c2ff1e673e64b51ba5cecb2fc015`. Exploration, sources and original prompts: `/mnt/data/keepsave-landing-design-2026-09-28/`.

## Scope / brief / RFC

Type-2 presentation change. Public React landing route, its scoped stylesheet, one generated hero asset and static no-JavaScript fallback. Preserve KeepSave identity, existing auth destinations, original SVG mark, Geist typography and amber/violet palette. No trust boundary moved: no auth, crypto, gateway, API, database, promotion policy or authenticated application changes. All examples are local, labeled and contain only key names and masking; no real credentials or account data are used. Existing app behavior remains owned by existing routes.

Story: software needs secret keys → loose copies become difficult to manage → put keys in a project vault → give tools access to a particular environment → compare changes before production → use existing developer tools / get started. One fictional storefront project connects the examples. Separate actual capabilities from illustrative state; configured production approval is not a universal guarantee. Do not claim that a gateway prevents every possible malicious disclosure.

Chosen art stays dominant in the hero; product UI begins in the next scroll. Generated art is static so the selected visual stays consistent and optional WebGL is not needed to read this page. Existing auth/workspace scene code is preserved. The original draft's control locations and sample statuses are adapted to a usable, accessible demonstration rather than reproduced as decorative buttons.

## Harness and verification plan

Inline implementation in isolated `feat/landing-event-horizon-20260928` worktree, based on fresh synchronized `develop` at `9e2b3ba`. Scope fits one context; the repo workflow's opt-in multi-agent runner is not needed for this presentation-only change. No sensitive surface or state-mutating handler is added, so backend audit-row and negative-auth gates are not triggered.

Run existing frontend tests, lint, app/widget builds. Add focused behavioral coverage for the new walkthrough and navigation, without testing static prose. Inspect actual desktop/mobile output against the selected draft. Check environment/step consistency, keyboard focus, mobile menu dismissal, real links, FAQs, image fallback, no-JavaScript content, reduced motion and horizontal reflow at 320/390/768/1024/1440/1920. Record test scope and unverified browsers in the validation report. Local verification is not owner visual acceptance or CI/release evidence.

Rollback: discard this isolated uncommitted landing diff/worktree after preserving any desired artifacts; the original checkout remains unchanged. Standard PR review is required before integration. No PR, remote CI, merge, push or release is implied.

## Implementation record

Hero source: `/home/santapong/.codex/generated_images/01a0e604-ef49-7da1-bbcd-0fbe0601bda9/exec-920d3a2d-a403-4ced-9126-1aa402ab67e1.png` (1536×1024). Exact prompt: `hero-prompt.txt`. WebP output `frontend/public/images/event-horizon.webp`, SHA-256 `23784b489128c2ad478b340a637127cbb57d9f56ecaea6274396b12d6cf11207`, approximately 108 KiB. The original SVG mark is reused unchanged.

Research used the earlier inspected homepages of Doppler, Infisical, Linear, Resend and Tailscale. Applied patterns: a concrete product surface immediately following the visual promise, one memorable art object, familiar use cases, restrained navigation and clear documentation access. Full observed patterns and limitations remain in the dated exploration's `design-notes.md`; no competitor claims or customer logos were copied.

The selected concept is now implemented with extended storytelling. See [verification and preview](../../validation/2026-09-28-landing/README.md) for actual checks and limits.

## Owner review

2026-09-28 follow-up: the owner said “Landing page is good now” and moved attention to login. Record the landing as visually accepted; no release or integration action was requested. Proposed login companion artifacts are separate at `/mnt/data/keepsave-login-design-2026-09-28/`; login application source was unchanged at that review. The later apply-now request is implemented separately in [the social login RFC](../2026-09-28-social-login.md).
