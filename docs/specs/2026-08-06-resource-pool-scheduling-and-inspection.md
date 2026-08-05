# Resource-pool scheduling and upstream inspection

Status: Accepted for implementation  
Date: 2026-08-06

## Goal

Make shared resource pools reuse the scheduling behavior already exposed by
ordinary groups, while keeping credentials, health, priority, weight, and
usage state shared across every protocol endpoint that uses the pool.

The same pool must be usable concurrently by different groups with different
selection strategies. Scheduling is a property of the group-to-pool binding,
not of the pool itself.

## Ownership model

- A resource pool owns physical credentials and shared credential state.
- A resource-pool endpoint owns one protocol type and Base URL.
- A group chooses a pool, one compatible endpoint, and a scheduling policy.
- Batch and File object ownership always overrides ordinary scheduling.

## Scheduling contract

`group.config.key_selection_strategy` is the authoritative default strategy
for both ordinary keys and pool resources:

- `round_robin`: smooth weighted round-robin in the first available priority
  tier; never reads or writes conversation affinity.
- `random`: weighted random selection in the first available priority tier.
- `sticky`: reuse a successful group-scoped affinity binding, otherwise choose
  by smooth weighted round-robin and bind only after success.
- `fill_first`: keep using the successful group-scoped resource until it is no
  longer selectable or `fill_max_consecutive_requests` is reached. It does not
  create an additional resource-pool cooldown state.

All strategies enforce hard priority tiers first. Weight only distributes work
inside the first tier that contains selectable credentials.

### Compatibility

- Ordinary groups without a configured strategy remain `round_robin`.
- Existing pool-bound groups are migrated to `sticky`, preserving the behavior
  that the current runtime actually applies even though the pool API reports
  `round_robin`.
- Existing affinity entries may be read once as a legacy fallback and are then
  rebound into the new group-scoped namespace after success.
- Pool-level affinity TTL and migration wait remain compatibility defaults.
  A pool-bound group may override them with
  `resource_affinity_ttl_seconds` and
  `resource_busy_wait_milliseconds`.
- The deprecated `resource_pools.strategy` column remains readable for schema
  compatibility but is no longer a user-facing or runtime policy source.

## Failure and object-routing invariants

- A credential failure changes the physical resource once and is immediately
  visible to all groups and endpoints using the pool.
- HTTP 404 and recognized request-shape failures do not damage credential
  health.
- A response may retry another resource only before streaming starts and only
  when the request is replay-safe.
- Batch/File follow-up requests always use the recorded owner resource and
  endpoint, regardless of the configured scheduling strategy.

## Management UI

- The scheduler card remains visible when a standard group binds a resource
  pool.
- The same four strategy names are used for ordinary and pooled credentials.
- Pool-bound sticky groups can configure affinity TTL and migration wait.
- Pool-bound fill-first groups configure the maximum consecutive successful
  requests; legacy cooldown-only controls are not shown for pooled resources.
- The resource-pool page no longer labels the entire pool as round-robin.
  Pool timing values are presented as compatibility defaults.

## Upstream inspection

Model discovery and balance inspection share a credential-resolution service:

- Resolve the selected endpoint.
- Select an active physical resource without changing conversation affinity.
- Decrypt explicitly; never send encrypted storage text upstream.
- Use the configured HTTP transport and bounded timeouts.
- Retry another active resource only when the inspection failure is
  credential-specific and replay-safe.

Model discovery is endpoint-scoped and reuses the existing group model UI.
Balance inspection is provider-adapter based. The first supported adapters are
DeepSeek, OpenRouter, SiliconFlow, and Moonshot/Kimi. Arbitrary balance URLs are
not accepted.

## Focused acceptance checks

1. Two groups bound to one pool can concurrently select with `sticky` and
   `round_robin` without affecting each other's scheduler state.
2. Random and fill-first selection respect priority and weight.
3. Existing pool-bound groups receive sticky compatibility behavior after the
   one-time migration; ordinary groups remain unchanged.
4. Pool-bound scheduler controls are visible and persist through group edits.
5. Batch/File ownership tests remain green.
6. Endpoint model discovery and supported balance adapters never expose raw
   credentials in API responses or logs.
7. Backend targeted tests, frontend type-check/build, and a container startup
   smoke check pass before release.

## Explicit non-goals for this change

- No fifth scheduling strategy such as least-connections.
- No automatic balance polling across every credential.
- No balance-driven automatic weight changes.
- No new downstream proxy URL solely to select a scheduling strategy.
- No destructive removal of compatibility database columns.
