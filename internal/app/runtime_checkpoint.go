package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gpt-load/internal/health"
	"gpt-load/internal/state"
)

const runtimeStateCheckpointFileName = "runtime-state.checkpoint.json"
const pendingResourceBindingsFileName = "resource-bindings.pending.json"
const resourceBindingImportTimeout = 3 * time.Second

// RuntimeStateCheckpoint persists best-effort mutable runtime state between
// process runs. Implementations must not become a startup or shutdown blocker.
type RuntimeStateCheckpoint interface {
	Restore(context.Context) error
	Save(context.Context) error
}

type runtimeStateCheckpointDocument struct {
	Credentials []state.CredentialRuntimeCheckpoint `json:"credentials,omitempty"`
	Stats       []health.StatsRuntimeCheckpoint     `json:"stats,omitempty"`
	Scheduling  *state.SchedulingCheckpoint         `json:"scheduling,omitempty"`
	Responses   []state.ResponseBinding             `json:"responses,omitempty"`
}

// FileRuntimeStateCheckpoint stores disposable runtime state in DATA_DIR.
// Native resource ownership is first staged separately for retryable database
// import. Malformed runtime snapshots are consumed instead of replayed.
type FileRuntimeStateCheckpoint struct {
	path             string
	registry         *state.CredentialRegistry
	stats            *health.StatsStore
	responseBindings *state.ResponseBindings
	resourceStore    ResourceBindingSaver
	removeFile       func(string) error
}

// ResourceBindingSaver persists native resource ownership restored from an
// older checkpoint, so bindings created before the resource table existed are
// not lost on the next unclean exit.
type ResourceBindingSaver interface {
	Claim(context.Context, state.ResponseBinding) (state.ResponseBinding, error)
}

// SetResourceBindingStore wires the persistent resource ownership store.
func (checkpoint *FileRuntimeStateCheckpoint) SetResourceBindingStore(store ResourceBindingSaver) {
	checkpoint.resourceStore = store
}

func NewFileRuntimeStateCheckpoint(
	dataDir string,
	registry *state.CredentialRegistry,
	stats *health.StatsStore,
	responseBindings *state.ResponseBindings,
) *FileRuntimeStateCheckpoint {
	return &FileRuntimeStateCheckpoint{
		path:             filepath.Join(dataDir, runtimeStateCheckpointFileName),
		registry:         registry,
		stats:            stats,
		responseBindings: responseBindings,
		removeFile:       os.Remove,
	}
}

// Restore consumes the checkpoint file if present. Failures are returned to
// the lifecycle owner, which continues startup with database-backed state.
func (checkpoint *FileRuntimeStateCheckpoint) Restore(ctx context.Context) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	pendingPath := filepath.Join(filepath.Dir(checkpoint.path), pendingResourceBindingsFileName)
	var pending []state.ResponseBinding
	if raw, err := os.ReadFile(pendingPath); err == nil {
		if err := json.Unmarshal(raw, &pending); err != nil {
			return fmt.Errorf("decode pending resource bindings: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read pending resource bindings: %w", err)
	}

	var document runtimeStateCheckpointDocument
	raw, err := os.ReadFile(checkpoint.path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read runtime state checkpoint: %w", err)
	}
	if err == nil {
		if decodeErr := json.Unmarshal(raw, &document); decodeErr != nil {
			if err := checkpoint.removeFile(checkpoint.path); err != nil {
				return fmt.Errorf("consume runtime state checkpoint: %w", err)
			}
			return fmt.Errorf("decode runtime state checkpoint: %w", decodeErr)
		}
		if checkpoint.resourceStore != nil {
			pending = mergePendingResourceBindings(pending, document.Responses)
			// Durable resource ownership must survive consumption of the disposable
			// runtime snapshot, including a crash during database import.
			if len(pending) > 0 {
				if err := writePendingResourceBindings(pendingPath, pending); err != nil {
					return err
				}
			}
		}
		if err := checkpoint.removeFile(checkpoint.path); err != nil {
			return fmt.Errorf("consume runtime state checkpoint: %w", err)
		}
		if checkpoint.registry != nil {
			checkpoint.registry.RestoreRuntimeCheckpoint(document.Credentials)
			if document.Scheduling != nil {
				checkpoint.registry.SchedulingState().RestoreCheckpoint(*document.Scheduling)
			}
		}
		if checkpoint.stats != nil {
			checkpoint.stats.RestoreRuntimeCheckpoint(document.Stats)
		}
	}
	bindings := document.Responses
	var importErr error
	if checkpoint.resourceStore != nil {
		bindings = nil
		for _, binding := range document.Responses {
			if !state.IsResourceBindingKey(binding.ResponseID) {
				bindings = append(bindings, binding)
			}
		}
		resources, err := checkpoint.persistResourceBindings(ctx, pendingPath, pending)
		bindings = append(bindings, resources...)
		importErr = err
	}
	if checkpoint.responseBindings != nil {
		if err := checkpoint.responseBindings.RestoreCheckpoint(bindings); err != nil {
			return errors.Join(importErr, err)
		}
	}
	return importErr
}

func mergePendingResourceBindings(pending, restored []state.ResponseBinding) []state.ResponseBinding {
	type identity struct {
		access            uint
		key               string
		group, credential uint
		generation        uint64
	}
	seen := make(map[identity]int)
	result := make([]state.ResponseBinding, 0, len(pending)+len(restored))
	now := time.Now()
	for _, source := range [][]state.ResponseBinding{pending, restored} {
		for _, binding := range source {
			if !state.IsResourceBindingKey(binding.ResponseID) || !binding.ExpiresAt.After(now) {
				continue
			}
			key := identity{binding.AccessKeyID, binding.ResponseID, binding.GroupID, binding.CredentialID, binding.IdentityGeneration}
			if index, exists := seen[key]; exists {
				if binding.ExpiresAt.Before(result[index].ExpiresAt) {
					result[index] = binding
				}
				continue
			}
			seen[key] = len(result)
			result = append(result, binding)
		}
	}
	return result
}

func writePendingResourceBindings(path string, bindings []state.ResponseBinding) error {
	payload, err := json.Marshal(bindings)
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".resource-bindings-pending-*")
	if err != nil {
		return fmt.Errorf("create pending resource bindings: %w", err)
	}
	defer os.Remove(file.Name())
	if _, err := file.Write(payload); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Rename(file.Name(), path); err != nil {
		return fmt.Errorf("save pending resource bindings: %w", err)
	}
	return nil
}

func (checkpoint *FileRuntimeStateCheckpoint) persistResourceBindings(ctx context.Context, path string, bindings []state.ResponseBinding) ([]state.ResponseBinding, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, resourceBindingImportTimeout)
	defer cancel()
	var restored, failed []state.ResponseBinding
	var firstFailure error
	failedCount := 0
	for index, binding := range bindings {
		if err := ctx.Err(); err != nil {
			failed = append(failed, bindings[index:]...)
			restored = append(restored, bindings[index:]...)
			failedCount += len(bindings) - index
			if firstFailure == nil {
				firstFailure = err
			}
			break
		}
		if !binding.ExpiresAt.After(time.Now()) {
			continue
		}
		canonical, err := checkpoint.resourceStore.Claim(ctx, binding)
		if err != nil {
			failedCount++
			if firstFailure == nil {
				firstFailure = err
			}
			if !errors.Is(err, state.ErrResourceBindingConflict) {
				failed = append(failed, binding)
				restored = append(restored, binding)
			}
			continue
		}
		restored = append(restored, canonical)
	}
	if len(failed) > 0 {
		if err := writePendingResourceBindings(path, failed); err != nil {
			failedCount++
			if firstFailure == nil {
				firstFailure = err
			}
		}
	} else if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		failedCount++
		if firstFailure == nil {
			firstFailure = err
		}
	}
	if firstFailure != nil {
		return restored, fmt.Errorf("import pending resource bindings (%d failed operations): %w", failedCount, firstFailure)
	}
	return restored, nil
}

// Save serializes the current runtime state and writes it directly to the
// fixed checkpoint file. A failed write is removed best-effort and returned so
// the lifecycle owner can log it without blocking shutdown.
func (checkpoint *FileRuntimeStateCheckpoint) Save(ctx context.Context) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	document := runtimeStateCheckpointDocument{}
	if checkpoint.registry != nil {
		document.Credentials = checkpoint.registry.CaptureRuntimeCheckpoint()
		scheduling := checkpoint.registry.SchedulingState().CaptureCheckpoint()
		document.Scheduling = &scheduling
	}
	if checkpoint.stats != nil {
		document.Stats = checkpoint.stats.CaptureRuntimeCheckpoint()
	}
	if checkpoint.responseBindings != nil {
		document.Responses = checkpoint.responseBindings.CaptureCheckpoint()
	}
	payload, err := json.Marshal(document)
	if err != nil {
		return err
	}
	if err := os.WriteFile(checkpoint.path, payload, 0o600); err != nil {
		_ = os.Remove(checkpoint.path)
		return err
	}
	return nil
}

func contextError(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	return ctx.Err()
}
