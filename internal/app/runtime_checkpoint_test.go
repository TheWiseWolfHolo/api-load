package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"gpt-load/internal/health"
	"gpt-load/internal/state"
	"gpt-load/internal/storage"

	"github.com/sirupsen/logrus"
)

type runtimeCheckpointFake struct {
	restore func(context.Context) error
	save    func(context.Context) error
}

type checkpointLifecycleLogHook struct {
	entries []*logrus.Entry
}

func (hook *checkpointLifecycleLogHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

func (hook *checkpointLifecycleLogHook) Fire(entry *logrus.Entry) error {
	if entry.Data["event"] == "startup.checkpoint_restore" {
		hook.entries = append(hook.entries, entry)
	}
	return nil
}

func (fake runtimeCheckpointFake) Restore(ctx context.Context) error {
	if fake.restore == nil {
		return nil
	}
	return fake.restore(ctx)
}

func (fake runtimeCheckpointFake) Save(ctx context.Context) error {
	if fake.save == nil {
		return nil
	}
	return fake.save(ctx)
}

func TestAppRestoresCheckpointAfterRuntimeRecovery(t *testing.T) {
	db, err := storage.Open(":memory:")
	if err != nil {
		t.Fatalf("storage.Open() error = %v", err)
	}
	var order []string
	checkpoint := runtimeCheckpointFake{
		restore: func(context.Context) error {
			order = append(order, "checkpoint")
			return nil
		},
	}
	application := NewApp(AppParams{
		Engine: mustNewEngine(t),
		Config: testConfig(t),
		DB:     db,
		StartupBootstrap: startupBootstrapFunc(func(context.Context) error {
			order = append(order, "bootstrap")
			return nil
		}),
		RuntimeState: runtimeStateLoaderFunc(func(context.Context) error {
			order = append(order, "runtime")
			return nil
		}),
		StartupRecovery: startupRecoveryFunc(func(context.Context) error {
			order = append(order, "recovery")
			return nil
		}),
		RuntimeCheckpoint: checkpoint,
		ControlRuntime:    newControlRuntimeFake(nil, false),
		RequestLogs:       newRequestLogRuntimeFake(nil, nil),
	})
	cleanupApp(t, application)

	if err := application.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if want := []string{"bootstrap", "runtime", "recovery", "checkpoint"}; !slices.Equal(order, want) {
		t.Fatalf("startup order = %#v, want %#v", order, want)
	}
}

func TestAppLogsCheckpointRestoreFailureOnce(t *testing.T) {
	dataDir := t.TempDir()
	path := filepath.Join(dataDir, runtimeStateCheckpointFileName)
	if err := os.WriteFile(path, []byte(`{"credentials":[`), 0o600); err != nil {
		t.Fatalf("write malformed checkpoint fixture: %v", err)
	}

	db, err := storage.Open(":memory:")
	if err != nil {
		t.Fatalf("storage.Open() error = %v", err)
	}
	application := NewApp(AppParams{
		Engine:            mustNewEngine(t),
		Config:            testConfig(t),
		DB:                db,
		StartupBootstrap:  startupBootstrapFunc(noopStartupBootstrap),
		RuntimeState:      runtimeStateLoaderFunc(func(context.Context) error { return nil }),
		RuntimeCheckpoint: NewFileRuntimeStateCheckpoint(dataDir, nil, nil, nil),
		ControlRuntime:    newControlRuntimeFake(nil, false),
		RequestLogs:       newRequestLogRuntimeFake(nil, nil),
	})
	cleanupApp(t, application)

	hook := &checkpointLifecycleLogHook{}
	logger := logrus.StandardLogger()
	previousHooks := make(logrus.LevelHooks, len(logger.Hooks))
	for level, hooks := range logger.Hooks {
		previousHooks[level] = append([]logrus.Hook(nil), hooks...)
	}
	logger.AddHook(hook)
	t.Cleanup(func() { logger.ReplaceHooks(previousHooks) })

	if err := application.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if len(hook.entries) != 1 {
		t.Fatalf("checkpoint restore lifecycle log count = %d, want 1", len(hook.entries))
	}
	if got := hook.entries[0].Level; got != logrus.WarnLevel {
		t.Fatalf("checkpoint restore lifecycle log level = %s, want warning", got)
	}
}

func TestAppSavesCheckpointBeforeRequestLogsAndUsesIndependentContext(t *testing.T) {
	db, err := storage.Open(":memory:")
	if err != nil {
		t.Fatalf("storage.Open() error = %v", err)
	}
	var order []string
	checkpoint := runtimeCheckpointFake{
		save: func(ctx context.Context) error {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			order = append(order, "checkpoint")
			return nil
		},
	}
	requestLogs := newRequestLogRuntimeFake(nil, func(context.Context) error {
		order = append(order, "request-logs")
		return nil
	})
	controlRuntime := newControlRuntimeFake(nil, true)
	application := NewApp(AppParams{
		Engine:            mustNewEngine(t),
		Config:            testConfig(t),
		DB:                db,
		StartupBootstrap:  startupBootstrapFunc(noopStartupBootstrap),
		RuntimeState:      runtimeStateLoaderFunc(func(context.Context) error { return nil }),
		RuntimeCheckpoint: checkpoint,
		ControlRuntime:    controlRuntime,
		RequestLogs:       requestLogs,
	})
	cleanupApp(t, application)
	t.Cleanup(controlRuntime.Release)

	if err := application.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := application.Stop(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Stop() error = %v, want context cancellation from forced drain", err)
	}
	if want := []string{"checkpoint", "request-logs"}; !slices.Equal(order, want) {
		t.Fatalf("shutdown order = %#v, want %#v", order, want)
	}
}

func TestFileRuntimeStateCheckpointRestoresAndConsumesFile(t *testing.T) {
	dataDir := t.TempDir()
	path := filepath.Join(dataDir, runtimeStateCheckpointFileName)

	registry := state.NewCredentialRegistry()
	if err := registry.ReplaceCredentials([]state.CredentialEntry{{
		ID: 1, GroupID: 10, Version: 1, IdentityGeneration: 1, Fingerprint: "test-1", Status: state.CredentialStatusActive,
		EncryptedValue: "cipher-one",
	}}); err != nil {
		t.Fatalf("replace registry: %v", err)
	}
	registry.SetCooldown(1, time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC))
	registry.SetBlacklisted(1)
	registry.IncrFailure(1)
	stats := health.NewStatsStore()
	stats.RecordFailure(1, health.FailureCategoryUpstreamHostError, 503, time.Date(2026, 8, 7, 11, 59, 0, 0, time.UTC))

	checkpoint := NewFileRuntimeStateCheckpoint(dataDir, registry, stats, nil)
	if err := checkpoint.Save(context.Background()); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("checkpoint file stat error = %v", err)
	}

	loadedRegistry := state.NewCredentialRegistry()
	if err := loadedRegistry.ReplaceCredentials([]state.CredentialEntry{{
		ID: 1, GroupID: 10, Version: 1, IdentityGeneration: 1, Fingerprint: "test-1", Status: state.CredentialStatusActive,
		EncryptedValue: "cipher-one",
	}, {
		ID: 2, GroupID: 20, Version: 1, IdentityGeneration: 2, Fingerprint: "test-2", Status: state.CredentialStatusActive,
		EncryptedValue: "cipher-two",
	}}); err != nil {
		t.Fatalf("replace loaded registry: %v", err)
	}
	loadedStats := health.NewStatsStore()
	loader := NewFileRuntimeStateCheckpoint(dataDir, loadedRegistry, loadedStats, nil)
	if err := loader.Restore(context.Background()); err != nil {
		t.Fatalf("Restore() error = %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("checkpoint file still exists after Restore(), stat error = %v", err)
	}

	entry := loadedRegistry.Snapshot()[0]
	if entry.ID != 1 || !entry.CooldownUntil.Equal(time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC)) ||
		!entry.Blacklisted || entry.FailureCount != 1 {
		t.Fatalf("restored key runtime state = %#v", entry)
	}
	gotStats := loadedStats.Snapshot(1, time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC))
	if gotStats.Failure != 1 || gotStats.Problem != 1 || gotStats.LastStatusCode != 503 {
		t.Fatalf("restored key stats = %#v", gotStats)
	}
}

func TestFileRuntimeStateCheckpointRestoresResponseOwnership(t *testing.T) {
	dir := t.TempDir()
	original := state.NewResponseBindings()
	if !original.Record(7, "stored-response", state.CredentialRef{ID: 2, GroupID: 3, IdentityGeneration: 4}) {
		t.Fatal("record failed")
	}
	want, _ := original.Lookup(7, "stored-response")
	checkpoint := NewFileRuntimeStateCheckpoint(dir, nil, nil, original)
	if err := checkpoint.Save(context.Background()); err != nil {
		t.Fatal(err)
	}
	restored := state.NewResponseBindings()
	loader := NewFileRuntimeStateCheckpoint(dir, nil, nil, restored)
	if err := loader.Restore(context.Background()); err != nil {
		t.Fatal(err)
	}
	got, ok := restored.Lookup(7, "stored-response")
	if !ok || got.AccessKeyID != want.AccessKeyID || got.CredentialID != want.CredentialID ||
		got.GroupID != want.GroupID || got.IdentityGeneration != want.IdentityGeneration ||
		!got.ExpiresAt.Equal(want.ExpiresAt) {
		t.Fatalf("restored binding = %#v, %t; want %#v", got, ok, want)
	}
}

type resourceBindingSaverFake struct {
	saved   []state.ResponseBinding
	err     error
	onClaim func(context.Context)
}

func (fake *resourceBindingSaverFake) Claim(ctx context.Context, binding state.ResponseBinding) (state.ResponseBinding, error) {
	fake.saved = append(fake.saved, binding)
	if fake.onClaim != nil {
		fake.onClaim(ctx)
	}
	return binding, fake.err
}

func TestPendingResourceImportStopsOnCancellationAndKeepsRemainingBindings(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, pendingResourceBindingsFileName)
	bindings := state.NewResponseBindings()
	ref := state.CredentialRef{ID: 2, GroupID: 3, IdentityGeneration: 4}
	for _, key := range []string{"native/mistral/files/first", "native/mistral/files/second"} {
		if !bindings.Record(7, key, ref) {
			t.Fatal("record failed")
		}
	}
	pending := bindings.CaptureCheckpoint()
	if err := writePendingResourceBindings(path, pending); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	saver := &resourceBindingSaverFake{onClaim: func(ctx context.Context) {
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) > resourceBindingImportTimeout {
			t.Error("import has no bounded overall deadline")
		}
		cancel()
	}}
	loader := NewFileRuntimeStateCheckpoint(dir, nil, nil, nil)
	loader.SetResourceBindingStore(saver)
	if _, err := loader.persistResourceBindings(ctx, path, pending); !errors.Is(err, context.Canceled) {
		t.Fatalf("import cancellation = %v", err)
	}
	if len(saver.saved) != 1 {
		t.Fatal("import kept querying the database after cancellation")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var remaining []state.ResponseBinding
	if err := json.Unmarshal(raw, &remaining); err != nil || len(remaining) != 1 || remaining[0].ResponseID != pending[1].ResponseID {
		t.Fatalf("unattempted resource was lost: %#v, %v", remaining, err)
	}
}

func TestFileRuntimeStateCheckpointPersistsRestoredResourceOwnership(t *testing.T) {
	dir := t.TempDir()
	original := state.NewResponseBindings()
	ref := state.CredentialRef{ID: 2, GroupID: 3, IdentityGeneration: 4}
	if !original.Record(7, "stored-response", ref) || !original.Record(7, "native/mistral/files/file-1", ref) {
		t.Fatal("record failed")
	}
	if err := NewFileRuntimeStateCheckpoint(dir, nil, nil, original).Save(context.Background()); err != nil {
		t.Fatal(err)
	}
	saver := &resourceBindingSaverFake{err: errors.New("database is down")}
	loader := NewFileRuntimeStateCheckpoint(dir, nil, nil, state.NewResponseBindings())
	loader.SetResourceBindingStore(saver)
	if err := loader.Restore(context.Background()); err == nil {
		t.Fatal("failed resource persistence was not reported")
	}
	if len(saver.saved) != 1 || saver.saved[0].ResponseID != "native/mistral/files/file-1" || saver.saved[0].CredentialID != 2 {
		t.Fatalf("persisted bindings = %#v; want only the native resource", saver.saved)
	}
	if _, ok := loader.responseBindings.Lookup(7, "stored-response"); !ok {
		t.Fatal("persistence failure dropped restored in-memory ownership")
	}
	pending := filepath.Join(dir, pendingResourceBindingsFileName)
	if _, err := os.Stat(pending); err != nil {
		t.Fatalf("resource persistence failure lost its durable retry file: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, runtimeStateCheckpointFileName)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("disposable runtime state was not consumed: %v", err)
	}
	saver.err = nil
	saver.saved = nil
	retry := NewFileRuntimeStateCheckpoint(dir, nil, nil, state.NewResponseBindings())
	retry.SetResourceBindingStore(saver)
	if err := retry.Restore(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(saver.saved) != 1 {
		t.Fatal("next startup did not retry pending resource ownership")
	}
	if _, ok := retry.responseBindings.Lookup(7, "stored-response"); ok {
		t.Fatal("pending resource import replayed ordinary runtime state")
	}
	if _, ok := retry.responseBindings.Lookup(7, "native/mistral/files/file-1"); !ok {
		t.Fatal("retried resource was not restored")
	}
	if _, err := os.Stat(pending); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("successful import left pending resources: %v", err)
	}
}

func TestFileRuntimeCheckpointDoesNotRestoreConflictingPersistentOwner(t *testing.T) {
	dir := t.TempDir()
	bindings := state.NewResponseBindings()
	if !bindings.Record(7, "native/mistral/files/conflict", state.CredentialRef{ID: 2, GroupID: 3, IdentityGeneration: 4}) {
		t.Fatal("record failed")
	}
	if err := NewFileRuntimeStateCheckpoint(dir, nil, nil, bindings).Save(t.Context()); err != nil {
		t.Fatal(err)
	}
	loader := NewFileRuntimeStateCheckpoint(dir, nil, nil, state.NewResponseBindings())
	loader.SetResourceBindingStore(&resourceBindingSaverFake{err: state.ErrResourceBindingConflict})
	if err := loader.Restore(t.Context()); !errors.Is(err, state.ErrResourceBindingConflict) {
		t.Fatalf("restore conflict = %v", err)
	}
	if _, ok := loader.responseBindings.Lookup(7, "native/mistral/files/conflict"); ok {
		t.Fatal("conflicting owner was published to memory")
	}
	if _, err := os.Stat(filepath.Join(dir, pendingResourceBindingsFileName)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("known conflicting owner remained queued for a later retry: %v", err)
	}
}

func TestFileRuntimeStateCheckpointReturnsErrorWhenDeleteFails(t *testing.T) {
	dataDir := t.TempDir()
	path := filepath.Join(dataDir, runtimeStateCheckpointFileName)
	raw, err := json.Marshal(runtimeStateCheckpointDocument{
		Credentials: []state.CredentialRuntimeCheckpoint{{ID: 1, GroupID: 10}},
	})
	if err != nil {
		t.Fatalf("marshal checkpoint fixture: %v", err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("write checkpoint fixture: %v", err)
	}

	registry := state.NewCredentialRegistry()
	if err := registry.ReplaceCredentials([]state.CredentialEntry{{
		ID: 1, GroupID: 10, Version: 1, IdentityGeneration: 1, Fingerprint: "test-1", Status: state.CredentialStatusActive,
		EncryptedValue: "cipher-one",
	}}); err != nil {
		t.Fatalf("replace registry: %v", err)
	}
	checkpoint := NewFileRuntimeStateCheckpoint(dataDir, registry, health.NewStatsStore(), nil)
	// The normal file implementation removes the file successfully. This test
	// documents that a failed removal must prevent applying stale data through
	// the injectable filesystem hook used by the implementation.
	checkpoint.removeFile = func(string) error { return os.ErrPermission }
	if err := checkpoint.Restore(context.Background()); !errors.Is(err, os.ErrPermission) {
		t.Fatalf("Restore() error = %v, want permission error", err)
	}
	if got := registry.Snapshot()[0].WeightManual; got != nil {
		t.Fatalf("configured weight after failed checkpoint removal = %v, want unset", got)
	}
}

func TestFileRuntimeStateCheckpointConsumesMalformedFileAndReturnsError(t *testing.T) {
	dataDir := t.TempDir()
	path := filepath.Join(dataDir, runtimeStateCheckpointFileName)
	if err := os.WriteFile(path, []byte(`{"credentials":[`), 0o600); err != nil {
		t.Fatalf("write malformed checkpoint fixture: %v", err)
	}
	registry := state.NewCredentialRegistry()
	if err := registry.ReplaceCredentials([]state.CredentialEntry{{
		ID: 1, GroupID: 10, Version: 1, IdentityGeneration: 1, Fingerprint: "test-1", Status: state.CredentialStatusActive,
		EncryptedValue: "cipher-one",
	}}); err != nil {
		t.Fatalf("replace registry: %v", err)
	}
	checkpoint := NewFileRuntimeStateCheckpoint(dataDir, registry, health.NewStatsStore(), nil)
	if err := checkpoint.Restore(context.Background()); err == nil {
		t.Fatal("Restore() error = nil, want malformed checkpoint error")
	}
	if got := registry.Snapshot()[0].WeightManual; got != nil {
		t.Fatalf("configured weight after malformed checkpoint = %v, want unset", got)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("malformed checkpoint file was not consumed, stat error = %v", err)
	}
}

func TestCorruptPendingBindingsDoNotBlockValidRuntimeCheckpoint(t *testing.T) {
	dir := t.TempDir()
	entries := []state.CredentialEntry{{ID: 1, GroupID: 3, Version: 1, IdentityGeneration: 1, Fingerprint: "synthetic", Status: state.CredentialStatusActive, EncryptedValue: "synthetic-cipher"}}
	registry := state.NewCredentialRegistry()
	if err := registry.ReplaceCredentials(entries); err != nil {
		t.Fatal(err)
	}
	registry.IncrFailure(1)
	bindings := state.NewResponseBindings()
	ref := state.CredentialRef{ID: 1, GroupID: 3, IdentityGeneration: 1}
	for _, key := range []string{"stored-response", "native/mistral/files/valid-main"} {
		if !bindings.Record(7, key, ref) {
			t.Fatal("record failed")
		}
	}
	if err := NewFileRuntimeStateCheckpoint(dir, registry, nil, bindings).Save(t.Context()); err != nil {
		t.Fatal(err)
	}
	partial := state.ResponseBinding{AccessKeyID: 7, ResponseID: "native/mistral/files/corrupt-partial", GroupID: 3, CredentialID: 1, IdentityGeneration: 1, ExpiresAt: time.Now().Add(time.Hour)}
	encoded, err := json.Marshal(partial)
	if err != nil {
		t.Fatal(err)
	}
	corrupt := append(append([]byte("["), encoded...), []byte(",false]")...)
	pendingPath := filepath.Join(dir, pendingResourceBindingsFileName)
	if err := os.WriteFile(pendingPath, corrupt, 0o600); err != nil {
		t.Fatal(err)
	}
	loadedRegistry := state.NewCredentialRegistry()
	if err := loadedRegistry.ReplaceCredentials(entries); err != nil {
		t.Fatal(err)
	}
	restored := state.NewResponseBindings()
	saver := &resourceBindingSaverFake{}
	loader := NewFileRuntimeStateCheckpoint(dir, loadedRegistry, nil, restored)
	loader.SetResourceBindingStore(saver)
	var decodeErr *json.UnmarshalTypeError
	if err := loader.Restore(t.Context()); !errors.As(err, &decodeErr) {
		t.Fatalf("restore must report the isolated corrupt file: %v", err)
	}
	if loadedRegistry.Snapshot()[0].FailureCount != 1 {
		t.Fatal("corrupt pending file blocked the valid runtime checkpoint")
	}
	for _, key := range []string{"stored-response", "native/mistral/files/valid-main"} {
		if _, found := restored.Lookup(7, key); !found {
			t.Fatalf("valid main binding %q was not restored", key)
		}
	}
	if len(saver.saved) != 1 || saver.saved[0].ResponseID != "native/mistral/files/valid-main" {
		t.Fatal("partially decoded corrupt pending bindings were imported")
	}
	backups, err := filepath.Glob(pendingPath + ".corrupt-*")
	if err != nil || len(backups) != 1 {
		t.Fatalf("corrupt file was not preserved separately: %v", err)
	}
	preserved, err := os.ReadFile(backups[0])
	if err != nil || !bytes.Equal(preserved, corrupt) {
		t.Fatalf("preserved corrupt bytes changed: %v", err)
	}
	if _, err := os.Stat(pendingPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("corrupt pending file remains active: %v", err)
	}
	retry := NewFileRuntimeStateCheckpoint(dir, nil, nil, state.NewResponseBindings())
	retry.SetResourceBindingStore(saver)
	if err := retry.Restore(t.Context()); err != nil {
		t.Fatalf("next restart was blocked by the same corrupt file: %v", err)
	}
	if _, err := os.Stat(backups[0]); err != nil {
		t.Fatalf("later restart removed the corrupt backup: %v", err)
	}
}

func TestCorruptPendingBindingsPreservationFailureKeepsOriginalFiles(t *testing.T) {
	dir := t.TempDir()
	pendingPath := filepath.Join(dir, pendingResourceBindingsFileName)
	corrupt := []byte("[")
	if err := os.WriteFile(pendingPath, corrupt, 0o600); err != nil {
		t.Fatal(err)
	}
	mainPath := filepath.Join(dir, runtimeStateCheckpointFileName)
	if err := os.WriteFile(mainPath, []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	loader := NewFileRuntimeStateCheckpoint(dir, nil, nil, nil)
	loader.renameFile = func(string, string) error { return os.ErrPermission }
	err := loader.Restore(t.Context())
	var decodeErr *json.SyntaxError
	if !errors.Is(err, os.ErrPermission) || !errors.As(err, &decodeErr) {
		t.Fatalf("rename and parse errors were not both reported: %v", err)
	}
	got, err := os.ReadFile(pendingPath)
	if err != nil || !bytes.Equal(got, corrupt) {
		t.Fatalf("failed rename changed corrupt original: %v", err)
	}
	if _, err := os.Stat(mainPath); err != nil {
		t.Fatalf("main checkpoint was consumed after failed preservation: %v", err)
	}
	if backups, err := filepath.Glob(pendingPath + ".corrupt-*"); err != nil || len(backups) != 0 {
		t.Fatalf("failed rename left an empty backup: %v", err)
	}
}

func TestCorruptPendingBindingsJoinLaterImportFailure(t *testing.T) {
	dir := t.TempDir()
	bindings := state.NewResponseBindings()
	if !bindings.Record(7, "native/mistral/files/import-failure", state.CredentialRef{ID: 1, GroupID: 3, IdentityGeneration: 1}) {
		t.Fatal("record failed")
	}
	if err := NewFileRuntimeStateCheckpoint(dir, nil, nil, bindings).Save(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, pendingResourceBindingsFileName), []byte("["), 0o600); err != nil {
		t.Fatal(err)
	}
	importFailure := errors.New("synthetic import failure")
	loader := NewFileRuntimeStateCheckpoint(dir, nil, nil, state.NewResponseBindings())
	loader.SetResourceBindingStore(&resourceBindingSaverFake{err: importFailure})
	err := loader.Restore(t.Context())
	var decodeErr *json.SyntaxError
	if !errors.As(err, &decodeErr) || !errors.Is(err, importFailure) {
		t.Fatalf("corruption and later import failure were not both reported: %v", err)
	}
}
