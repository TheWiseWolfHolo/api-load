# ADR 0002: Keep api-load as the provider scheduler upstream of New API

- Status: Accepted
- Date: 2026-08-29

## Context

api-load and GPT-Load v2 overlap in credential management, protocol routing, request logs, usage views, and management UI. Replacing api-load with GPT-Load v2 or turning api-load into another user-facing distribution platform would discard api-load's resource-pool semantics and duplicate capabilities already owned by New API.

The production relationship is New API as the user-facing distribution gateway, api-load as its provider scheduler, and official providers or relays behind api-load.

## Decision

api-load remains the sole provider scheduler and the source of truth for upstream credentials, shared health, affinity, retries, cooldowns, and protocol endpoints. New API continues to own users, client tokens, quotas, billing, and model distribution.

GPT-Load v2 is a reference implementation and feature donor, not a runtime dependency or migration target. Features and UI patterns may be reimplemented in api-load when they improve provider operations, but they must be translated into api-load's domain model rather than importing GPT-Load's Group, Channel, Credential, or AccessKey model.

The data-plane contract consumed by New API remains backward compatible: proxy URLs, proxy-key authentication, model identifiers, request and response formats, and established error semantics do not change without a separate coordinated decision. Management APIs, internal implementation, and the operator UI may evolve independently.

Bug fixes and product/UI work are delivered as separate changes so that behavior regressions can be attributed and rolled back independently.

## Rejected alternatives

### Migrate api-load to GPT-Load v2

Rejected because GPT-Load v2 cannot represent api-load's shared resource-pool state and would make New API depend on a less specialized scheduler.

### Expand api-load into a lightweight New API

Rejected because user management, balances, payments, and customer token lifecycle already belong to New API. Duplicating them would create competing sources of truth.

### Copy GPT-Load v2 features and pages directly

Rejected because visual similarity does not preserve domain meaning. Adopted features must use api-load terminology, permissions, resource relationships, and existing Vue management stack.

## Consequences

- Feature evaluation is based on whether it strengthens provider scheduling or operator workflows.
- New API user and billing features remain out of scope for api-load.
- GPT-Load v2 can heavily influence information architecture, diagnostics, monitoring, and interaction design without controlling api-load's schema.
- Existing New API integrations continue to work while the api-load management surface changes incrementally.
