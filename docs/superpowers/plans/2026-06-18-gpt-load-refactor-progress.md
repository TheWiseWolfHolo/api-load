# GPT-Load Refactor Progress

Source spec: `docs/superpowers/specs/2026-06-18-gpt-load-refactor-tdd-spec.md`

## Baseline Verification

Branch: `refactor-gpt-load-tdd`

Environment setup:

- Installed frontend dependencies with `npm ci`.
- Installed Go 1.25.0 to user-local path `C:/Users/Holo/.local/go/go1.25.0/bin`.

Baseline results:

- `npm run type-check`: PASS.
- `npm run lint:check`: PASS with existing Prettier CRLF warnings.
- `npm run build`: PASS with existing Vite chunk-size warning.
- `go test ./...`: initial run failed before `web/dist` existed; after `npm run build`, PASS.

## Milestone Results

Results will be appended after each phase verification.

### Phase 1: Key Management And Migration Foundation

Implemented:

- `disabled` key status across backend models and frontend types.
- Disabled keys excluded from active key selection, automatic recovery, restore-all, cron validation, and default manual validation.
- Explicit single-key and batch status update APIs.
- Key list filtering for `all`, `active`, `invalid`, and `disabled`; notes/search matching.
- Key statistics split `invalid_keys` and `disabled_keys`.
- Structured import parser for legacy text, JSON string arrays, JSONL, and CSV.
- Duplicate import policies: `keep`, `update_notes`, `update_status`, and `overwrite`.
- JSONL export with notes/status and JSONL import/export round-trip.
- Frontend disabled status filter/badge/export option and local provider icon metadata/assets.

Verification:

- `go test ./...`: PASS.
- `npm run type-check`: PASS.
- `npm run lint:check`: PASS with existing CRLF Prettier warnings.
- `npm run build`: PASS with existing Vite chunk-size warning.

## Final README Verification

README files were updated only after Phase 1 through Phase 4 verification passed.

Final verification after README updates:

- `go test ./...`: PASS.
- `npm run type-check`: PASS.
- `npm run lint:check`: PASS with existing CRLF Prettier warnings.
- `npm run build`: PASS with existing Vite chunk-size warning.

### Phase 2: Model Discovery And Scheduler Core

Implemented:

- OpenAI-compatible, OpenRouter-normalized, Gemini, and Anthropic manual-only model discovery service.
- Group model selection persistence and management APIs for save, fetch, and discovery.
- Aggregate group model summaries with deterministic deduplicated model union and source sub-group metadata.
- Group config scheduler fields for `round_robin`, `random`, `sticky`, and `fill_first`, including validation and API-visible config options.
- Default round-robin compatibility through the original active-list rotation path.
- Random selection with deterministic test RNG injection.
- Sticky key affinity for group, model, and model plus proxy-key scopes, with raw proxy keys hashed in store keys.
- Fill-first current-key reuse, request-limit switching, transient 429 cooldown, and quota/billing error classification.
- Proxy request path integration for model/proxy-key-aware selection and fill-first result recording.
- Frontend types and API wrappers for model and scheduler configuration.

Verification:

- `go test ./...`: PASS.
- `npm run type-check`: PASS.
- `npm run lint:check`: PASS with existing CRLF Prettier warnings.
- `npm run build`: PASS with existing Vite chunk-size warning.

### Phase 3: Model Mapping, Aggregate Routing, And Token Usage

Implemented:

- Model mapping resolver for exact aliases, wildcard aliases, strict rejection, and non-strict fallback.
- Deterministic weighted target selection with injectable test RNG and positive-weight filtering.
- Aggregate external model lists that expose aliases plus manual aggregate model overrides while excluding weight-zero targets.
- `model_mappings` group field surfaced in backend responses and frontend types.
- Request log token usage fields for input, output, total, cache read, cache write, thinking tokens, and usage source.
- Upstream and estimated token usage helpers that keep estimated usage separate and do not overwrite upstream-reported usage.
- Non-stream proxy response usage extraction for request logs.
- Dashboard token stats API grouped by model, group, or hour with fixed safe grouping expressions.
- Frontend types and dashboard API wrapper for token stats.

Verification:

- `go test ./...`: PASS.
- `npm run type-check`: PASS.
- `npm run lint:check`: PASS with existing CRLF Prettier warnings.
- `npm run build`: PASS with existing Vite chunk-size warning.

### Phase 4: Proxy Pool, Full Migration, And Production Backpressure

Implemented:

- Proxy policy resolver with key/group/system/env precedence, direct mode, fixed proxy resolution, disabled item rejection, and masked proxy display.
- Proxy pool selector strategies for round-robin, random, sticky, and failover over enabled proxy items only.
- Shared key and URL credential masking, including invalid proxy URL log sanitization and repository dummy-secret scan.
- Full-system migration envelope and import/export round-trip preservation for group scalar fields, upstreams, config, header rules, model redirect rules, model lists, model mappings, and full-system key counters.
- Export modes for plain confirmation, encrypted, masked, and config-only key safety.
- Import preview for new keys, duplicate keys, notes updates, and overwritten groups without database mutation.
- Request log buffering with batch flush, emergency flush, hard-limit dropped-log accounting, and security-warning preservation.
- Frontend model management panel for discovery, search, manual add, select all, invert selection, and save.
- Frontend scheduler controls that show relevant fields for round-robin, random, sticky, and fill-first strategies.
- Frontend proxy URL display masking for credentialed proxy config values.

Verification:

- `go test ./...`: PASS.
- `npm run type-check`: PASS.
- `npm run lint:check`: PASS with existing CRLF Prettier warnings.
- `npm run build`: PASS with existing Vite chunk-size warning.
