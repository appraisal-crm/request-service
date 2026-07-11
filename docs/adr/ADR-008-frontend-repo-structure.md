# ADR-008: Frontend as a Single Monorepo

> English version · [Русская версия](i18n/ru/ADR-008-frontend-repo-structure.md)

**Status:** Accepted  
**Date:** 2026-07-12

## Context

The frontend is React + TypeScript and serves four roles: client, appraiser,
inspector, admin. These roles share a large surface:

- **Auth** — the same Keycloak OAuth2/OIDC flow, token handling and route guards.
- **API-client types** — DTOs generated from each backend service's
  `swagger.json`; all apps consume the same request/inspect contracts.
- **Design system** — a shared UI kit, theming and i18n.
- **Tooling** — TypeScript, ESLint, Prettier and build config.

Two structural questions had to be settled: (1) one repo or many, and (2) how many
deployable apps, split along which axis.

The backend is polyrepo (each Go service deploys independently — see
[ADR-003](ADR-003-database-per-service.md)), and the earlier charter mechanically
mirrored that as "four separate SPA repositories, one per role". But roles are not
the right split axis, and four repos force either code duplication or a fifth
"shared" package published to a registry — friction with no upside for a small team.

The meaningful boundary is **external vs internal**, not per-role: `client` is a
public-facing portal (self-service: submit and track a request), while
appraiser/inspector/admin are **internal staff tools** that differ only in which
features they see. Splitting staff tools per role gains nothing — the same app can
show/hide features from the JWT `realm_access.roles` claim. But keeping the public
client portal separate from the staff app is worth it: the public bundle should not
ship internal UI/logic, and external onboarding/auth differs.

## Decision

The frontend is a **single monorepo** — `github.com/appraisal-crm/appraisal-frontend`,
managed with **pnpm workspaces + Turborepo** — split into two deployable apps along
the external/internal boundary:

```
frontend/
  apps/
    client/       # EXTERNAL public portal — request-service (submit / track a request)
    office/       # INTERNAL staff app, role-gated in-app — request-service + inspect-service
                  #   appraiser: statuses, assign inspector
                  #   inspector: fill inspection, photos, complete
                  #   admin:     added when its backend exists
  packages/
    ui/           # shared design system / component library
    api-client/   # typed API clients generated from services' swagger.json
    auth/         # Keycloak OIDC (Auth Code + PKCE), token store, role-based guards
    tsconfig/     eslint-config/                                # shared tooling
```

Within `office`, roles are gated in-app from the token's `realm_access.roles` claim
— not by separate apps. Each app under `apps/` is independently buildable and
deployable; the monorepo is a build-time/source-organisation choice, not runtime
coupling. Shared code lives in `packages/`, consumed via the workspace, not a
registry.

This decision applies to the **frontend only**. The backend stays polyrepo;
[ADR-003](ADR-003-database-per-service.md) is unaffected — a frontend monorepo
introduces no runtime coupling between backend services.

## Consequences

**Positive:**
- Shared auth, API types and UI live in one place — no duplication, no cross-repo
  version-bump dance; atomic changes across apps and packages land in one PR.
- The public `client` bundle stays free of internal staff UI and logic.
- New staff roles are a matter of in-app gating, not a new app/repo.
- One toolchain and CI pipeline; Turborepo caches and scopes builds to affected apps.

**Negative:**
- The repo grows larger; contributors clone both apps even to touch one.
- CI must scope work to affected apps (Turborepo affected-graph) to stay fast.
- Per-app access control is coarser than separate repos (org-level, not per-repo).
- `office` must enforce role gating rigorously — a bug there exposes features across
  roles within the internal app (the external client portal stays isolated).
