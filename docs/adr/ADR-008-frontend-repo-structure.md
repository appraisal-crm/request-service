# ADR-008: Frontend as a Single Monorepo

> English version · [Русская версия](i18n/ru/ADR-008-frontend-repo-structure.md)

**Status:** Accepted  
**Date:** 2026-07-12

## Context

The system has four React + TypeScript single-page apps — one per role: client,
appraiser, inspector, admin. They are distinct deployables with different
audiences, but they share a large surface:

- **Auth** — the same Keycloak OAuth2/OIDC flow, token handling and route guards.
- **API-client types** — DTOs generated from each backend service's
  `swagger.json`; all four apps consume the same request/inspect contracts.
- **Design system** — a shared UI kit, theming and i18n so the four apps look and
  behave consistently.
- **Tooling** — TypeScript, ESLint, Prettier and build config.

The backend is polyrepo (each Go service is an independently deployable module —
see [ADR-003](ADR-003-database-per-service.md)), and the earlier charter carried
that split to the frontend as "four separate SPA repositories". For the shared
frontend surface above, four repos force either code duplication or a fifth
"shared" package published to a registry and version-bumped across four
consumers — friction with no upside for a small team.

## Decision

The frontend is a **single monorepo**: `github.com/appraisal-crm/appraisal-frontend`,
managed with **pnpm workspaces + Turborepo**.

```
appraisal-frontend/
  apps/
    client/       appraiser/       inspector/       admin/     # 4 SPAs, deployed independently
  packages/
    ui/           # shared design system / component library
    api-client/   # typed API clients generated from services' swagger.json
    auth/         # Keycloak OIDC flow, token store, route guards
    tsconfig/     eslint-config/                                # shared tooling
```

Each app under `apps/` remains an **independently buildable and deployable**
artifact — the monorepo is a build-time/source-organisation choice, not a runtime
coupling. Shared code lives in `packages/` and is consumed via the workspace, not
a registry.

This decision applies to the **frontend only**. The backend stays polyrepo;
[ADR-003](ADR-003-database-per-service.md) (database-per-service, no cross-service
DB access) is unaffected — a frontend monorepo introduces no runtime coupling
between backend services.

## Consequences

**Positive:**
- Shared auth, API types and UI live in one place — no duplication, no
  cross-repo version-bump dance.
- Atomic changes across apps and shared packages land in one PR/commit.
- One toolchain, lint/test/build config and CI pipeline (Turborepo caches builds).
- A contract change in a shared package immediately type-checks against all four
  consumers.

**Negative:**
- The repo grows larger; contributors clone all four apps even to touch one.
- CI must scope work to affected apps (Turborepo affected-graph) to stay fast.
- Per-app access control is coarser than separate repos (org-level, not per-repo).
- Requires monorepo discipline (workspace boundaries, no cross-app imports except
  through `packages/`).
