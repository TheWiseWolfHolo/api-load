# Domain Context

## Purpose

api-load is the single scheduler for official upstream credentials. It exposes protocol-specific groups to downstream gateways while keeping credential health, affinity, usage, and failover decisions in one place.

## Core terms

### Resource pool

A resource pool is the ownership boundary for a set of shared credentials. It owns scheduling policy, affinity TTL, busy-wait behavior, and optional quota auto-restore settings.

### Shared credential

A shared credential is one upstream API key inside a resource pool. Its enabled state, health state, priority, weight, usage counters, failure counters, cooldown, and affinity are global within that pool.

The same credential is never duplicated merely because it can be used through multiple API formats or base URLs.

### Protocol endpoint

A protocol endpoint belongs to one resource pool and defines:

- a human-readable name;
- one channel type such as `openai` or `anthropic`;
- one Base URL, including an optional fixed path prefix;
- whether the endpoint is enabled.

An endpoint contains no credentials and performs no scheduling.

### Group

A standard group is a downstream route. It either:

- uses its legacy in-group upstreams and keys; or
- binds one resource pool and one enabled endpoint whose channel type matches the group.

Legacy upstream configuration remains dormant while a pool binding is active and becomes usable again when the pool is unbound.

### Object binding

An object binding records the resource pool, protocol endpoint, and shared credential that created an account-scoped Batch or File object. Later operations must return to that exact endpoint and credential.

## Relationships

```text
Resource Pool
├── Protocol Endpoint (OpenAI) ──┐
├── Protocol Endpoint (Anthropic)├── selected explicitly by Groups
└── Shared Credentials           ┘
    ├── Key A + global state
    ├── Key B + global state
    └── Key C + global state
```

## Invariants

1. A raw key is unique within a resource pool, regardless of endpoint or Base URL.
2. A group bound to a pool must also bind exactly one enabled endpoint matching its channel type.
3. Endpoint selection happens before credential scheduling; retries may change the credential but not the endpoint.
4. Any upstream failure except HTTP 404 updates the shared credential globally. With auto-restore disabled, the credential becomes invalid immediately.
5. HTTP 404 does not count as a credential failure and does not change credential health.
6. A successful request is the only event that increments the credential success counter.
7. Batch/File operations never migrate away from the endpoint and credential recorded at object creation.
8. A referenced pool, endpoint, or credential cannot be permanently deleted.
9. Management APIs never return raw credential values after creation.

## Legacy migration

Existing URL-plus-key rows are migrated by:

1. creating protocol endpoints from distinct legacy URLs and bound group channel types;
2. binding each existing group to a deterministic preferred endpoint;
3. deduplicating rows by key hash;
4. conservatively merging health and usage state;
5. moving Batch/File ownership to the surviving credential and selected endpoint.

The deprecated `upstream_resources.upstream_url` column remains for schema compatibility but is cleared by migration and ignored by runtime routing.

If a legacy pool has no bound groups, its URLs are preserved as `legacy` endpoints. An administrator must assign each one a supported channel type before a group can bind it.
