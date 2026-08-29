# GPT-Load v2 adoption matrix

## Purpose

GPT-Load v2 is a feature and interaction reference for api-load. It is not a migration target, runtime dependency, or schema source. Every adopted idea must preserve api-load's role as the provider credential scheduler behind New API.

## Decision matrix

| GPT-Load v2 idea | Decision | api-load translation | Delivery phase |
| --- | --- | --- | --- |
| Credential status and operational actions | Adopt and strengthen | Keep operator enablement separate from runtime health; add validate-and-recover, classified batch results, and explicit force recovery | 0 |
| Credential diagnostics | Adopt | Show last success/failure, cooldown, failure reason, usage, and the validation route without revealing raw keys | 1 |
| Request attempt timeline | Adapt | Group retries and the final response into one request trace while preserving api-load's current downstream response and error contract | 2 |
| Log filters and detail drawers | Adopt | Add provider-scheduler filters such as pool, endpoint, credential, model, status, retry stage, and error class | 2 |
| Dashboard information architecture | Adapt | Focus on upstream capacity, healthy schedulable credentials, failure pressure, retries, cooldowns, and route health rather than customer revenue | 3 |
| Channel creation guidance | Adapt | Build a group + endpoint binding wizard around api-load's pool, endpoint, and validation-route model | 3 |
| Settings grouping and inheritance | Adopt | Make system defaults, group overrides, and effective values visible without changing current data-plane behavior | 3 |
| Access-key portal | Reject | New API already owns downstream users and client tokens | — |
| User balance, quota, and billing | Reject | New API remains the source of truth for customer distribution and accounting | — |
| Group / Channel / Credential schema | Reject | It cannot represent api-load's shared resource pools, protocol endpoints, affinity, and object bindings without semantic loss | — |
| Direct frontend copy | Reject | Reuse information architecture and interaction lessons, but implement them with api-load's Vue, Naive UI, terminology, and domain contracts | — |

## Delivery order

1. Correct recovery semantics and make the two state axes visible.
2. Add credential-level diagnostics using the recovery surface as the reference interaction pattern.
3. Add request traces that connect retries, credential choices, and the final downstream result.
4. Rework dashboard, settings, and group creation around provider operations.

## Compatibility gate

No phase may change New API-facing proxy URLs, proxy-key authentication, exposed model IDs, request or response formats, or error semantics without a separate compatibility decision and rollout plan.
