# ADR 0003: Separate operator enablement from runtime health

- Status: Accepted
- Date: 2026-08-29

## Context

A credential can be healthy but intentionally disabled by an administrator, or enabled by an administrator but rejected by its upstream provider. Treating both conditions as one status makes automatic failure handling overwrite operator intent and makes a generic "Enable" action silently return known-bad credentials to scheduling.

The current backend separates enablement from health, but the management UI and bulk actions do not express the distinction consistently. Bulk enablement does not recover automatically invalidated credentials, while some recovery paths can reset status without first validating the upstream credential.

## Decision

Operator enablement and runtime health remain independent state dimensions.

- Enable and disable actions change only operator intent.
- A successful validation may restore runtime health but must not enable a manually disabled credential.
- Normal recovery of an unhealthy credential performs a real validation and returns the credential to scheduling only when validation succeeds.
- Bulk recovery validates every selected credential and reports classified outcomes instead of treating the batch as one undifferentiated success.
- Force recovery without validation is a separate, explicitly dangerous administrative action.
- Successful recovery clears consecutive failure state, cooldown state, and the current failure reason while retaining historical counters.

## Rejected alternatives

### Make Enable restore health automatically

Rejected because it would return credentials with known authentication, quota, or account failures to scheduling without evidence that they recovered.

### Collapse enablement and health into one status

Rejected because automatic failures would then overwrite administrator intent, and successful validation could silently re-enable credentials that an administrator deliberately disabled.

### Let every recovery action force active immediately

Rejected because an unverified reset creates retry storms and hides persistent upstream failures.

## Consequences

- The UI must display operator enablement separately from runtime health and cooldown.
- Resource tables need distinct Enable, Validate and recover, Bulk validate and recover, and Force recover actions.
- Recovery APIs need structured per-credential results for success, persistent invalidity, request-shape rejection, network failure, and skipped manual disablement.
- Existing scheduling semantics remain intact while operator recovery becomes predictable and auditable.
