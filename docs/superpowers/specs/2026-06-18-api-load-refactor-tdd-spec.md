# API-Load Refactor TDD Spec

> **For agentic workers:** REQUIRED SUB-SKILL before production changes: use `superpowers:test-driven-development`. Implementation plans should use `superpowers:subagent-driven-development` or `superpowers:executing-plans` task-by-task. Every behavior change below starts with a failing test, verifies the failure reason, then adds minimal production code.

**Goal:** Turn API-Load from a round-robin key proxy into a model-aware, cache-friendly, proxy-controllable, migration-ready AI API scheduler without rewriting the existing Go backend or Vue 3 / Naive UI frontend.

**Architecture:** Keep the current Go service boundaries, but add explicit services for key import/export, key selection strategies, model discovery, model mapping, proxy policies, and system migration. Keep the current proxy request path and group cache model intact, adding narrow interfaces where new behavior needs to plug in. Frontend work should extend the existing `Keys.vue` page and `web/src/components/keys/*` components before introducing new top-level routes.

**Tech Stack:** Go 1.25, Gin, Gorm, Redis or memory store, Vue 3, TypeScript, Naive UI, Vite, existing i18n files.

## Source Evidence

- Original requirement file: `E:/Script/api-load/apiload重构需求.md`.
- Local checkout: `E:/Script/api-load/repo`.
- Current upstream HEAD observed on 2026-06-18: `5579930 docs: blacklist_threshold (#419)`.
- Current key status constants are only `active` and `invalid` in `internal/models/types.go`.
- Current key selection rotates `group:{id}:active_keys` in `internal/keypool/provider.go`.
- Current key import accepts legacy text and JSON string arrays through `internal/services/key_service.go`.
- Current key export streams plain keys only through `internal/services/key_service.go`.
- Current frontend provider display uses emoji in `web/src/components/keys/GroupList.vue`.

## Global Constraints

- Do not rewrite the project. Extend the existing Go backend plus Vue 3 / Naive UI frontend.
- Keep `round_robin` as the default behavior for existing groups.
- Keep existing management and proxy APIs compatible unless a new versioned API is explicitly introduced.
- New features must be modular, configurable, and safe to disable.
- Redis store and memory store must behave the same for key selection, cooldown, active pools, and cache invalidation.
- No API response, UI view, log line, exported masked file, or test fixture may expose a real API key or proxy password.
- Plain key export is allowed only through an explicit user-confirmed export mode.
- Tests must use dummy keys and dummy proxy URLs.
- Every production behavior change requires a failing test first.
- Required verification before claiming a phase complete: `go test ./...`, `npm run type-check`, `npm run lint:check`, and `npm run build`.

## Test Naming

Use stable test IDs in test names, commit messages, and implementation plans:

- `KEY-*`: key state, key listing, key restore, validation, status statistics.
- `IMP-*`: key import parsing and duplicate policies.
- `EXP-*`: key export formats and export safety.
- `SCH-*`: key selection strategies.
- `MOD-*`: model discovery and model selection.
- `MAP-*`: model mapping and aggregate model routing.
- `PRX-*`: proxy policy and proxy pool behavior.
- `MIG-*`: full system import/export migration.
- `TOK-*`: token usage and cost statistics.
- `LOG-*`: request log backpressure and secret-safe logging.
- `UI-*`: frontend behavior, provider icons, i18n, responsive management views.
- `SEC-*`: security invariants that span backend, frontend, logs, and exports.

## Phase Boundaries

### Phase 1: Key Management And Migration Foundation

This phase is the minimum useful fork. It covers `disabled` status, notes-aware import/export, local provider icons, and basic UI cleanup. It does not include model discovery, fill-first scheduling, proxy pools, or token dashboards.

### Phase 2: Model Discovery And Scheduler Core

This phase adds model discovery, group model selection, `key_selection_strategy`, `random`, `sticky`, `fill_first`, and model affinity. It must keep Phase 1 import/export compatible.

### Phase 3: Model Mapping, Aggregate Routing, And Token Usage

This phase adds alias routing, wildcard and strict model mapping, aggregate group model summaries, and token usage collection/dashboard support.

### Phase 4: Proxy Pool, Full Migration, And Production Backpressure

This phase adds proxy pools, key/group proxy policies, full-system import/export, encrypted/masked export modes, and request log backpressure.

## Phase 1 TDD Specs

### Key Status

#### KEY-001: `disabled` is a persistent key status

**Target tests:** `internal/models/key_status_test.go`, `web/src/types/models.test.ts` if a frontend test runner is added.

Given an API key record with status `disabled`, when it is read from the database or serialized to API JSON, then the status remains exactly `disabled`.

Acceptance:

- Backend constants include `KeyStatusDisabled = "disabled"`.
- Frontend `KeyStatus` includes `"disabled"`.
- Existing `active` and `invalid` JSON values remain unchanged.

#### KEY-002: disabled keys are not loaded into the active key pool

**Target test:** `internal/keypool/provider_test.go`.

Given a group with one active, one invalid, and one disabled key, when `LoadKeysFromDB` rebuilds store state, then only the active key ID is present in `group:{id}:active_keys`.

Acceptance:

- Memory store and Redis-backed store follow the same active-list rules.
- Disabled key details may be cached in `key:{id}`, but must not be selectable.

#### KEY-003: disabled keys are never selected for proxy traffic

**Target test:** `internal/keypool/provider_test.go`.

Given a disabled key exists in the store hash and is absent from the active list, when `SelectKey(groupID)` is called repeatedly, then the disabled key is never returned.

Acceptance:

- If every key is disabled or invalid, `ErrNoActiveKeys` is returned.
- No fallback may query disabled keys directly from the database.

#### KEY-004: successful validation does not auto-enable disabled keys

**Target test:** `internal/keypool/provider_test.go`.

Given a key has status `disabled`, when `UpdateStatus(key, group, true, "")` or the underlying success handler runs, then status remains `disabled`, failure count is not used to re-add it to the active list, and no active list push occurs.

Acceptance:

- Invalid keys may still recover to active when validation succeeds.
- Disabled keys require an explicit enable operation.

#### KEY-005: restore-all only restores invalid keys

**Target test:** `internal/keypool/provider_test.go`.

Given a group has invalid and disabled keys, when `RestoreKeys(groupID)` runs, then only invalid keys become active and disabled keys remain disabled.

Acceptance:

- Result counts include only restored invalid keys.
- Disabled keys are not added to `group:{id}:active_keys`.

#### KEY-006: manual enable and disable are explicit operations

**Target tests:** `internal/services/key_service_test.go`, `internal/handler/key_handler_test.go`.

Given an active key, when the user disables it through the management API, then its status becomes `disabled`, failure count is preserved, and it is removed from the active list.

Given a disabled key, when the user enables it through the management API, then its status becomes `active`, failure count resets to `0`, and it is added to the active list.

Acceptance:

- Single-key and batch operations are supported.
- API validates requested status values.
- Batch result includes changed and ignored counts.

#### KEY-007: key list filters include disabled and all

**Target tests:** `internal/services/key_service_test.go`, `internal/handler/key_handler_test.go`.

Given keys in active, invalid, and disabled states, when listing keys with `status=active`, `status=invalid`, `status=disabled`, or no status, then results match the requested filter.

Acceptance:

- `status=all` and empty status both include all persisted statuses.
- Unknown status values return validation errors.

#### KEY-008: disabled keys are excluded from automatic cron validation

**Target test:** `internal/keypool/cron_checker_test.go`.

Given a group with invalid and disabled keys, when `CronChecker` submits validation jobs, then only invalid keys are validated.

Acceptance:

- Disabled keys are not decrypted for cron validation.
- `last_validated_at` still updates when no invalid keys exist.

#### KEY-009: manual group validation never implicitly restores disabled keys

**Target test:** `internal/services/key_manual_validation_service_test.go`.

Given manual validation is started without a status filter, when the service queries keys for validation, then disabled keys are excluded unless a future explicit `include_disabled=true` API is designed.

Acceptance:

- Existing active and invalid validation flows remain available.
- Disabled cannot be passed as a status to `ValidateGroupKeys` in Phase 1.

#### KEY-010: key statistics distinguish invalid and disabled

**Target tests:** `internal/services/group_service_test.go`, `internal/handler/dashboard_handler_test.go`.

Given a group with active, invalid, and disabled keys, when group stats and dashboard stats are requested, then active, invalid, disabled, and total counts are correct.

Acceptance:

- Existing consumers that only read `active_keys` and `invalid_keys` remain compatible.
- New `disabled_keys` field is added where frontend needs it.

### Notes Search, Import, And Export

#### KEY-011: notes search matches key notes without requiring raw key input

**Target test:** `internal/services/key_service_test.go`.

Given keys have notes, when listing with `notes=<keyword>` or `search=<keyword>`, then keys whose notes contain the keyword are returned.

Acceptance:

- Existing exact key hash search still works.
- Notes search is parameterized and does not use string-built SQL.

#### IMP-001: legacy text import remains compatible

**Target test:** `internal/services/key_import_parser_test.go`.

Given a text payload with one key per line, comma-separated keys, and whitespace-separated keys, when parsing import input, then all non-empty keys are returned with empty notes, active status, and inherit proxy policy.

Acceptance:

- Existing plain text behavior does not regress.
- Empty tokens are ignored.

#### IMP-002: JSON array of strings remains compatible

**Target test:** `internal/services/key_import_parser_test.go`.

Given `["sk-a","sk-b"]`, when parsing import input, then two import records are returned with keys `sk-a` and `sk-b`.

Acceptance:

- This format keeps the old parser behavior.
- Invalid JSON falls through to legacy text parsing only when it is not a supported structured format.

#### IMP-003: JSONL import supports notes and status

**Target test:** `internal/services/key_import_parser_test.go`.

Given JSONL rows containing `key`, `notes`, and `status`, when parsing import input, then notes and status are preserved per row.

Acceptance:

- Status accepts `active`, `invalid`, and `disabled`.
- Missing status defaults to `active`.
- Unknown status returns a parse error that names the row number and masks the key.

#### IMP-004: CSV import supports key, notes, and status headers

**Target test:** `internal/services/key_import_parser_test.go`.

Given a CSV file with headers `key,notes,status`, when parsing import input, then records preserve key, notes, and status.

Acceptance:

- Header names are case-insensitive.
- CSV rows without a key are rejected with row numbers.
- Notes longer than 255 runes return validation errors.

#### IMP-005: duplicate policy `keep` preserves existing records

**Target test:** `internal/services/key_import_service_test.go`.

Given an imported key already exists in the group, when duplicate policy is `keep`, then existing notes, status, failure count, and proxy policy remain unchanged.

Acceptance:

- Result includes duplicate count.
- No active-list churn occurs for kept rows.

#### IMP-006: duplicate policy `update_notes` updates only notes

**Target test:** `internal/services/key_import_service_test.go`.

Given an imported key already exists with old notes and invalid status, when duplicate policy is `update_notes`, then notes change and status remains invalid.

Acceptance:

- Empty imported notes intentionally clear notes only when `allow_empty_notes=true`; otherwise empty imported notes are ignored.

#### IMP-007: duplicate policy `update_status` updates only status

**Target test:** `internal/services/key_import_service_test.go`.

Given an imported key already exists with notes, when duplicate policy is `update_status`, then status changes, notes remain unchanged, and active list membership is updated according to the new status.

Acceptance:

- Updating to disabled removes the key from active selection.
- Updating to active adds it to active selection.

#### IMP-008: duplicate policy `overwrite` updates editable fields

**Target test:** `internal/services/key_import_service_test.go`.

Given an imported key already exists, when duplicate policy is `overwrite`, then notes, status, and Phase 4 proxy policy fields are overwritten while request counters and key hash remain intact.

Acceptance:

- Request count and created timestamp are not reset.
- Key value is not re-encrypted unless the raw key differs by hash.

#### EXP-001: legacy txt export remains available

**Target test:** `internal/services/key_export_service_test.go`.

Given a group has keys, when exporting with format `txt`, then output is one decrypted key per line.

Acceptance:

- Status filtering supports `all`, `active`, `invalid`, and `disabled`.
- Existing `/keys/export` behavior remains compatible when no format is specified.

#### EXP-002: JSONL export includes notes and status

**Target test:** `internal/services/key_export_service_test.go`.

Given keys have notes and statuses, when exporting with format `jsonl`, then each row contains `key`, `notes`, and `status`.

Acceptance:

- Rows do not include internal database IDs by default.
- Decryption failures skip the row and increment an export error count.

#### EXP-003: exported JSONL round-trips through import

**Target test:** `internal/services/key_import_export_roundtrip_test.go`.

Given a JSONL export from one group, when importing into an empty group, then keys, notes, and statuses match the source group.

Acceptance:

- Request counters are not copied by keys-only export.
- Disabled keys remain disabled after import.

### Provider Icons And Basic UI

#### UI-001: provider metadata uses local icons

**Target tests:** `web/src/utils/providerMeta.test.ts` once a frontend test runner is introduced.

Given channel types `openai`, `openai-response`, `gemini`, `anthropic`, `openrouter`, `deepseek`, `qwen`, `xai`, and `azure-openai`, when provider metadata is requested, then each provider resolves to a display name and local `/provider-icons/*.svg` or `.png` path.

Acceptance:

- No remote image URLs are returned.
- Unknown providers return a local default provider icon.

#### UI-002: group list does not render provider emoji

**Target tests:** frontend component test or Playwright screenshot assertion.

Given the group list renders standard groups for all supported channel types, when the group icon area is inspected, then it contains an image element or icon component backed by local assets, not emoji text.

Acceptance:

- The UI remains usable in light and dark mode.
- Icon alt text or title uses provider display name.

#### UI-003: key status badges distinguish active, invalid, and disabled

**Target tests:** frontend component test or Playwright screenshot assertion.

Given the key table displays keys with all three statuses, when rendered, then active uses success styling, invalid uses error or warning styling, and disabled uses neutral grey styling.

Acceptance:

- Status text is localized in Chinese, English, and Japanese.
- Long notes and masked keys do not overflow their card or table cell.

## Phase 2 TDD Specs

### Model Discovery And Selection

#### MOD-001: OpenAI-compatible model discovery calls `/v1/models`

**Target test:** `internal/services/model_discovery_service_test.go`.

Given an OpenAI-compatible group with active keys and upstream URL `https://api.example.com`, when model discovery runs, then it sends a request to `https://api.example.com/v1/models` and parses `data[].id`.

Acceptance:

- The first successful active key supplies the model list.
- Failed key attempts are summarized without exposing the key.

#### MOD-002: provider URL normalization handles OpenRouter

**Target test:** `internal/services/model_discovery_service_test.go`.

Given upstream URL `https://openrouter.ai`, when discovery runs for an OpenAI-compatible provider, then the normalized base becomes `https://openrouter.ai/api` before appending `/v1/models`.

Acceptance:

- URLs already ending in `/api` are not double-appended.
- Trailing slash variants normalize identically.

#### MOD-003: Gemini discovery calls `/v1beta/models` and strips `models/`

**Target test:** `internal/services/model_discovery_service_test.go`.

Given a Gemini group whose discovery response includes `models/gemini-2.5-pro`, when parsing models, then the saved model ID is `gemini-2.5-pro`.

Acceptance:

- Non-prefixed Gemini names are preserved.
- Empty model names are ignored.

#### MOD-004: Anthropic discovery is explicit manual-only in this phase

**Target test:** `internal/services/model_discovery_service_test.go`.

Given an Anthropic group, when automatic discovery is requested, then the service returns a typed unsupported error instructing the UI to allow manual model entry.

Acceptance:

- This is not treated as a failed key validation.
- Error messages do not include secret headers.

#### MOD-005: group model selection persists enabled models

**Target tests:** `internal/services/group_service_test.go`, `internal/handler/group_handler_test.go`.

Given a model list is saved for a standard group, when the group is fetched, then `models` returns the enabled model IDs in deterministic order.

Acceptance:

- Duplicate models are removed.
- Manual additions are trimmed and validated as non-empty strings.

#### MOD-006: aggregate groups summarize sub-group models

**Target test:** `internal/services/aggregate_group_service_test.go`.

Given an aggregate group with two sub-groups whose model lists overlap, when aggregate model summary is requested, then the response contains a deduplicated union with source sub-group metadata.

Acceptance:

- Disabled or weight-zero sub-groups do not contribute by default.
- UI can still manually override final exposed models.

### Key Selection Strategy Core

#### SCH-001: default round-robin behavior is unchanged

**Target test:** `internal/keypool/selector_round_robin_test.go`.

Given a group has no `key_selection_strategy`, when keys are selected repeatedly, then selection rotates through active keys using the existing store rotation behavior.

Acceptance:

- Existing active list keys remain the source of truth.
- Existing retry behavior continues to use the next selected key.

#### SCH-002: random strategy selects only active keys

**Target test:** `internal/keypool/selector_random_test.go`.

Given a random strategy selector with deterministic test RNG and active plus invalid plus disabled keys, when selecting keys, then only active keys can be returned.

Acceptance:

- Random selection can be tested deterministically by injecting RNG.
- Empty active set returns `ErrNoActiveKeys`.

#### SCH-003: sticky by group reuses the same key until invalidated

**Target test:** `internal/keypool/selector_sticky_test.go`.

Given `key_selection_strategy=sticky` and `key_affinity_scope=group`, when multiple requests select for the same group, then the same active key is returned until it becomes invalid, disabled, or deleted.

Acceptance:

- Sticky state works in memory and Redis stores.
- Switching after invalidation chooses another active key.

#### SCH-004: sticky by model keeps separate model affinities

**Target test:** `internal/keypool/selector_sticky_test.go`.

Given `key_affinity_scope=model`, when requests for model A and model B select keys, then each model gets its own sticky key slot.

Acceptance:

- Same group plus same model returns same key.
- Same group plus different model may return a different key.

#### SCH-005: sticky by model plus proxy key isolates proxy entrypoints

**Target test:** `internal/keypool/selector_sticky_test.go`.

Given `key_affinity_scope=model+proxy_key`, when the same model is requested through different proxy keys, then affinity keys are isolated by proxy key.

Acceptance:

- Proxy key values are hashed or otherwise safe in store keys.
- Store keys do not include raw proxy credentials or API keys.

#### SCH-006: fill-first keeps current key until a switch condition

**Target test:** `internal/keypool/selector_fill_first_test.go`.

Given `key_selection_strategy=fill_first`, when the current key succeeds repeatedly and no limit is reached, then the same key is returned for subsequent requests.

Acceptance:

- Switch conditions include configured status codes, quota patterns, max consecutive requests, max consecutive tokens, sticky TTL, and manual disable.
- Default max consecutive request and token limits of `0` mean unlimited.

#### SCH-007: fill-first sends transient rate limits to cooldown, not permanent exhausted

**Target test:** `internal/keypool/selector_fill_first_test.go`.

Given the current key receives a plain 429 without configured quota or billing patterns, when failure handling runs, then the key enters cooldown for `fill_cooldown_minutes` and is not marked exhausted.

Acceptance:

- Cooldown may live in Redis or memory and does not require a database status in Phase 2.
- Expired cooldown keys can become eligible again if still active.

#### SCH-008: exhausted is used only for explicit quota or billing failures

**Target test:** `internal/keypool/quota_error_classifier_test.go`.

Given upstream errors containing `insufficient_quota`, `quota_exceeded`, `billing_hard_limit`, or configured quota patterns, when classified, then the key can be marked exhausted.

Acceptance:

- Generic 429, 500, 502, and 503 do not become exhausted without matching quota patterns.
- Exhausted behavior is configurable before any database migration uses it.

#### SCH-009: scheduler config lives on group config and is API-visible

**Target tests:** `internal/services/group_service_test.go`, `web/src/types/models.test.ts` if frontend test runner exists.

Given a group config includes scheduler settings, when the group is saved and fetched, then `key_selection_strategy`, `key_affinity_scope`, and fill-first settings round-trip.

Acceptance:

- Missing settings default to `round_robin`.
- Unknown strategies and scopes return validation errors.

## Phase 3 TDD Specs

### Model Mapping And Aggregate Routing

#### MAP-001: exact model alias maps to weighted targets

**Target test:** `internal/services/model_mapping_service_test.go`.

Given alias `gpt-4.1` maps to two targets with weights 10 and 5, when resolving a request for `gpt-4.1`, then only targets with positive weight are candidates and the selected target includes sub-group ID and real model.

Acceptance:

- Selection is deterministic under injected test RNG.
- Returned target is safe to feed into aggregate routing.

#### MAP-002: wildcard aliases match configured model patterns

**Target test:** `internal/services/model_mapping_service_test.go`.

Given wildcard aliases `gpt-4*`, `claude-*`, and `gemini-*`, when resolving matching requested models, then the correct mapping rule is selected.

Acceptance:

- Exact aliases take precedence over wildcard aliases.
- Ambiguous wildcard precedence is deterministic by configured order.

#### MAP-003: strict mode rejects unmatched models

**Target test:** `internal/services/model_mapping_service_test.go`.

Given strict mode is true and no alias or wildcard matches, when resolving a model, then a typed not-supported error is returned.

Acceptance:

- Proxy response is a clear client error.
- The error does not trigger key failure count.

#### MAP-004: non-strict mode falls back to default routing

**Target test:** `internal/services/model_mapping_service_test.go`.

Given strict mode is false and no mapping matches, when resolving a model, then the resolver returns a fallback decision that keeps the original model and existing aggregate routing.

Acceptance:

- Existing model redirect rules remain compatible.
- Fallback does not hide configuration errors in strict mode.

#### MAP-005: aggregate model list can expose aliases instead of raw child names

**Target test:** `internal/services/aggregate_group_service_test.go`.

Given an aggregate group has model mappings, when external model list is requested, then exposed models include aliases and exclude disabled weight-zero targets.

Acceptance:

- Manual aggregate model overrides can include aliases and raw models.
- Duplicate aliases are removed.

### Token Usage

#### TOK-001: request logs store reported token usage

**Target test:** `internal/services/request_log_service_test.go`.

Given an upstream response includes usage fields, when request logging runs, then input, output, total, cache read, cache write, thinking, and source fields are stored.

Acceptance:

- Missing usage fields store zero values and `token_usage_source=none`.
- Existing request log reads remain compatible.

#### TOK-002: estimated token usage is separate from upstream reported usage

**Target test:** `internal/services/request_log_service_test.go`.

Given token usage is estimated locally, when logging runs, then estimated tokens are recorded and source is `estimated`, not `upstream`.

Acceptance:

- Estimated values never overwrite upstream-reported values in the same request.
- Dashboard can filter by usage source.

#### TOK-003: dashboard aggregates token usage by model, group, and time range

**Target test:** `internal/handler/dashboard_handler_test.go`.

Given request logs across groups, models, and timestamps, when token stats are requested, then totals match the selected time range and grouping.

Acceptance:

- Cache read and cache write tokens are visible separately.
- Thinking tokens are visible separately.

## Phase 4 TDD Specs

### Proxy Policy And Proxy Pool

#### PRX-001: proxy policy precedence is key, group, system, env

**Target test:** `internal/proxy/policy_resolver_test.go`.

Given key, group, system, and environment proxy settings all exist, when resolving proxy policy, then key policy wins over group, group wins over system, and system wins over environment.

Acceptance:

- `direct` mode bypasses lower-level proxy settings.
- Missing policy means inherit.

#### PRX-002: fixed proxy resolves a single configured proxy

**Target test:** `internal/proxy/policy_resolver_test.go`.

Given a fixed proxy policy references proxy `us-1`, when resolving, then the HTTP client uses that proxy URL.

Acceptance:

- Disabled proxy items are rejected.
- API and logs return masked proxy URLs.

#### PRX-003: proxy pool round-robin, random, sticky, and failover are selectable

**Target test:** `internal/proxy/pool_selector_test.go`.

Given a proxy pool with enabled proxy items, when selecting under each strategy, then selection follows the configured strategy and never returns disabled items.

Acceptance:

- Random uses injectable deterministic RNG in tests.
- Sticky strategies use safe affinity keys.

#### PRX-004: proxy credentials are masked everywhere outside transport setup

**Target tests:** `internal/proxy/mask_test.go`, `internal/handler/proxy_pool_handler_test.go`.

Given proxy URL `http://user:pass@host:8080`, when it is shown in API responses, UI data, logs, or export masked mode, then password and username are not exposed.

Acceptance:

- Allowed display examples are `http://host:8080` or `http://***@host:8080`.
- Transport setup still receives the original secret URL internally.

### Full Import/Export

#### MIG-001: full-system export contains versioned migration envelope

**Target test:** `internal/services/system_export_service_test.go`.

Given groups, keys, models, proxy pools, model mappings, settings, upstreams, header rules, redirects, and parameter overrides exist, when exporting `full_system`, then output includes `version`, `exported_at`, `groups`, `proxy_pools`, and `system_settings`.

Acceptance:

- Exported timestamps use RFC3339 with offset.
- Export structure is deterministic for snapshot testing.

#### MIG-002: export modes enforce key safety

**Target test:** `internal/services/system_export_service_test.go`.

Given export mode is `plain`, `encrypted`, `masked`, or `config_only`, when export runs, then keys are represented according to the mode.

Acceptance:

- `plain` requires explicit confirmation from the caller.
- `masked` output cannot be imported as real keys.
- `config_only` contains no key material.

#### MIG-003: full-system import preview reports planned changes

**Target test:** `internal/services/system_import_service_test.go`.

Given an import file overlaps existing groups, keys, and proxy pools, when preview runs, then it reports counts for new keys, duplicate keys, notes updates, new proxy pools, and overwritten groups.

Acceptance:

- Preview does not mutate database or store state.
- Preview masks keys and proxy credentials.

#### MIG-004: full-system import/export round-trip preserves supported fields

**Target test:** `internal/services/system_import_export_roundtrip_test.go`.

Given a system export is imported into a fresh database, when groups and keys are fetched, then supported fields match source values.

Acceptance:

- Key request counters are preserved only in full-system mode.
- Key notes, status, models, mappings, proxy policies, and header rules are preserved.

### Request Log Backpressure

#### LOG-001: request logs flush in batches

**Target test:** `internal/services/request_log_service_test.go`.

Given request log batching is enabled, when logs are enqueued below the emergency threshold, then they flush on interval or batch size.

Acceptance:

- Flush errors are visible in logs.
- No request path blocks indefinitely on log writes.

#### LOG-002: emergency flush triggers when pending logs exceed threshold

**Target test:** `internal/services/request_log_service_test.go`.

Given pending logs exceed the configured emergency threshold, when enqueue runs, then emergency flush is triggered.

Acceptance:

- Concurrent enqueue does not race or double-count pending logs.
- Flush result updates pending count accurately.

#### LOG-003: dropped log count is recorded when hard limit is exceeded

**Target test:** `internal/services/request_log_service_test.go`.

Given pending logs exceed the hard limit, when new logs arrive, then some logs may be dropped and dropped count increments.

Acceptance:

- Dropping logs never drops security warnings.
- Dropped count is visible in metrics or logs.

## Cross-Cutting Security Specs

#### SEC-001: key masking is consistent

**Target tests:** `internal/utils/string_utils_test.go`, frontend display utility tests.

Given API keys with OpenAI, Gemini, Anthropic, and unknown formats, when masked for display, logs, or masked export, then only a small prefix and suffix are visible.

Acceptance:

- Full keys appear only in in-memory transport setup and explicit plain export.
- Failed decryption messages do not include encrypted or decrypted key values.

#### SEC-002: logs never contain proxy passwords

**Target test:** log hook test around proxy policy and request failure paths.

Given a proxy URL contains credentials and a request fails, when logs are captured, then username and password do not appear.

Acceptance:

- Masking applies to structured fields and formatted messages.

#### SEC-003: tests use dummy secrets only

**Target check:** repository scan in CI or local verification script.

Given test fixtures and docs are scanned, when looking for realistic secret prefixes beyond dummy examples, then no real credential material is present.

Acceptance:

- Allowed examples include `sk-test-*`, `AIzaSyDummy*`, and `http://user:pass@example.invalid:8080`.

## Frontend Management Specs

#### UI-004: key management page supports efficient status workflows

**Target tests:** frontend component test or Playwright flow.

Given a selected standard group, when the key management page loads, then the user can filter by all, active, invalid, and disabled; edit notes; test keys; enable or disable keys; import; and export.

Acceptance:

- Controls are accessible by label or tooltip.
- Operations refresh counts without full page reload.

#### UI-005: model management UI supports discovery, search, manual add, and save

**Target tests:** frontend component test or Playwright flow.

Given a standard group, when the model management area opens, then the user can run discovery, search results, select all, invert selection, manually add a model, and save enabled models.

Acceptance:

- Discovery failures show clear messages without key leakage.
- Saved model count is visible in group details.

#### UI-006: scheduler UI exposes only relevant fields per strategy

**Target tests:** frontend component test or Playwright flow.

Given strategy `round_robin`, `random`, `sticky`, or `fill_first`, when selected in group settings, then only relevant configuration fields are shown.

Acceptance:

- Fill-first shows cooldown, switch status codes, quota patterns, max consecutive requests, max consecutive tokens, and sticky TTL.
- Invalid numeric values are rejected before submit.

#### UI-007: proxy pool UI masks proxy URLs

**Target tests:** frontend component test or Playwright flow.

Given proxy pool items with credentialed URLs, when rendered in the proxy pool page, then credentials are masked and test/edit/delete controls work against proxy IDs.

Acceptance:

- Raw proxy passwords never appear in DOM text.
- Test results show health status and recent failure time.

## Data And API Contract Summary

### API Key

Required fields:

- `id`
- `group_id`
- `key_value` for existing authenticated management list responses unless a later API deliberately switches to masked-only values
- `key_hash`
- `status`: `active`, `invalid`, or `disabled`
- `notes`
- `request_count`
- `failure_count`
- `last_used_at`
- `created_at`
- `updated_at`

Future Phase 4 field:

- `proxy_policy`

### Group

Required additions by phase:

- Phase 2: `models`, `key_selection_strategy`, `key_affinity_scope`, fill-first config fields.
- Phase 3: `model_mappings`, aggregate exposed model configuration.
- Phase 4: `proxy_policy`.

Backward compatibility:

- Groups without new fields behave as `round_robin`, no explicit models, inherit proxy, and no model mapping.

### Proxy Pool Item

Required Phase 4 fields:

- `id`
- `name`
- `url` stored encrypted or otherwise protected
- `masked_url` for API/UI
- `enabled`
- `notes`
- `health_status`
- `last_failed_at`
- `created_at`
- `updated_at`

## Required Verification Commands

Run these before claiming any phase complete:

```powershell
go test ./...
Set-Location web
npm run type-check
npm run lint:check
npm run build
```

For UI-heavy phases, also run browser verification against desktop and mobile viewports and capture evidence that no key, token, or proxy credential appears in rendered text.

## Coverage Map

- Original Section 2 Key management: `KEY-001` through `KEY-011`, `IMP-001` through `IMP-008`, `EXP-001` through `EXP-003`.
- Original Section 3 Key scheduling: `SCH-001` through `SCH-009`.
- Original Section 4 Model discovery and selection: `MOD-001` through `MOD-006`, `UI-005`.
- Original Section 5 Model mapping and aggregate routing: `MAP-001` through `MAP-005`.
- Original Section 6 Proxy and proxy pool: `PRX-001` through `PRX-004`, `UI-007`.
- Original Section 7 Full import/export: `MIG-001` through `MIG-004`.
- Original Section 8 UI refactor: `UI-001` through `UI-007`.
- Original Section 9 Provider icons: `UI-001`, `UI-002`.
- Original Section 10 Token usage: `TOK-001` through `TOK-003`.
- Original Section 11 Production readiness: `LOG-001` through `LOG-003`, `SEC-001` through `SEC-003`.
- Original Section 12 Implementation requirements: global constraints, phase boundaries, and required verification commands.

## Implementation Discipline

- For each test ID, write the failing test first and run the narrow test command.
- Verify the failure is caused by missing behavior, not compile errors or fixture mistakes.
- Add the smallest production change that makes the test pass.
- Run the narrow test again.
- Refactor only after green.
- Run the phase verification commands before marking the phase complete.
- Do not merge Phase 2 scheduler changes until Phase 1 disabled status and import/export tests are green.
- Do not merge Phase 4 proxy or migration changes until masking tests are green.


