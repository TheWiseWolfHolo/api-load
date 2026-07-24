# ADR 0001: Separate shared credentials from protocol endpoints

- Status: Accepted
- Date: 2026-07-24

## Context

One official credential set may support both OpenAI-compatible and Anthropic APIs through different Base URLs. The previous model stored URL and key as one scheduling unit, so representing both protocols duplicated the same key and split its health, cooldown, usage, and affinity state.

Groups need protocol-specific routing, while credential failures must remain visible to every group using the same purchased capacity.

## Decision

Resource pools own shared credentials and their global scheduling state. A resource pool may also own multiple protocol endpoints. Each endpoint defines one channel type and Base URL but owns no keys.

A pool-bound group explicitly stores both `resource_pool_id` and `resource_endpoint_id`. The endpoint must belong to the selected pool, be enabled, and match the group channel type. If exactly one endpoint matches, the server may select it automatically; ambiguous matches require an explicit choice.

Retries and affinity operate on credential IDs within the pool. Endpoint selection is fixed for the request. Batch/File ownership stores both endpoint ID and credential ID.

Every upstream failure except HTTP 404 affects the shared credential globally. Existing quota auto-restore remains opt-in.

## Consequences

- OpenAI and Anthropic groups can share one key set without duplicating state.
- Disabling or invalidating a key immediately removes it from all endpoints.
- A Base URL can be edited or disabled independently from credentials.
- Groups with multiple matching endpoints must make an explicit routing choice.
- Existing URL-plus-key data requires a one-time, conservative migration.
- The legacy upstream URL column remains temporarily for database compatibility but has no runtime meaning.

## Alternatives considered

### Duplicate each key per URL

Rejected because health, cooldown, statistics, and affinity diverge for the same purchased credential.

### Let groups store arbitrary Base URLs while pools only store keys

Rejected because endpoint lifecycle and validation would be scattered across groups, making reuse and administration harder.

### Select endpoints dynamically on every retry

Rejected because a retry should fail over capacity, not silently change API format or upstream path semantics.
