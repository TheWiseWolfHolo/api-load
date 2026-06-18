package services

import (
	"api-load/internal/models"
	"bufio"
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestEXP001LegacyTxtExportSupportsStatusFilters(t *testing.T) {
	svc, db, _, _ := newTestKeyService(t)
	group := createServiceTestGroup(t, db)
	seedKey(t, svc, group.ID, "sk-test-active", "active notes", models.KeyStatusActive, 0, 0)
	seedKey(t, svc, group.ID, "sk-test-disabled", "disabled notes", models.KeyStatusDisabled, 0, 0)

	var out bytes.Buffer
	result, err := svc.ExportKeysToWriter(group.ID, models.KeyStatusDisabled, "txt", &out)
	if err != nil {
		t.Fatalf("export txt: %v", err)
	}
	if result.ExportedCount != 1 || result.ErrorCount != 0 {
		t.Fatalf("unexpected export result: %#v", result)
	}
	if strings.TrimSpace(out.String()) != "sk-test-disabled" {
		t.Fatalf("unexpected txt export: %q", out.String())
	}
}

func TestEXP002JSONLExportIncludesNotesAndStatus(t *testing.T) {
	svc, db, _, _ := newTestKeyService(t)
	group := createServiceTestGroup(t, db)
	seedKey(t, svc, group.ID, "sk-test-jsonl", "paused", models.KeyStatusDisabled, 0, 0)

	var out bytes.Buffer
	result, err := svc.ExportKeysToWriter(group.ID, "all", "jsonl", &out)
	if err != nil {
		t.Fatalf("export jsonl: %v", err)
	}
	if result.ExportedCount != 1 || result.ErrorCount != 0 {
		t.Fatalf("unexpected export result: %#v", result)
	}

	var row map[string]string
	if err := json.Unmarshal(bytes.TrimSpace(out.Bytes()), &row); err != nil {
		t.Fatalf("decode JSONL row: %v output=%q", err, out.String())
	}
	if row["key"] != "sk-test-jsonl" || row["notes"] != "paused" || row["status"] != models.KeyStatusDisabled {
		t.Fatalf("unexpected JSONL row: %#v", row)
	}
	if _, hasID := row["id"]; hasID {
		t.Fatalf("JSONL export should not include database id: %#v", row)
	}
}

func TestEXP003JSONLExportRoundTripsThroughImport(t *testing.T) {
	sourceSvc, sourceDB, _, _ := newTestKeyService(t)
	sourceGroup := createServiceTestGroup(t, sourceDB)
	seedKey(t, sourceSvc, sourceGroup.ID, "sk-test-roundtrip-active", "primary", models.KeyStatusActive, 9, 12)
	seedKey(t, sourceSvc, sourceGroup.ID, "sk-test-roundtrip-disabled", "paused", models.KeyStatusDisabled, 3, 4)

	var out bytes.Buffer
	if _, err := sourceSvc.ExportKeysToWriter(sourceGroup.ID, "all", "jsonl", &out); err != nil {
		t.Fatalf("export jsonl: %v", err)
	}

	targetSvc, targetDB, _, _ := newTestKeyService(t)
	targetGroup := createServiceTestGroup(t, targetDB)
	records, err := ParseKeyImportInput(out.String())
	if err != nil {
		t.Fatalf("parse exported JSONL: %v", err)
	}
	if _, err := targetSvc.ImportKeyRecords(targetGroup.ID, records, KeyImportOptions{}); err != nil {
		t.Fatalf("import exported JSONL: %v", err)
	}

	rows := make(map[string]models.APIKey)
	scanner := bufio.NewScanner(strings.NewReader(out.String()))
	for scanner.Scan() {
		var row struct {
			Key string `json:"key"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &row); err != nil {
			t.Fatalf("decode exported row: %v", err)
		}
		var key models.APIKey
		if err := targetDB.Where("key_hash = ?", targetSvc.EncryptionSvc.Hash(row.Key)).First(&key).Error; err != nil {
			t.Fatalf("load imported key %s: %v", row.Key, err)
		}
		rows[row.Key] = key
	}

	if rows["sk-test-roundtrip-disabled"].Status != models.KeyStatusDisabled || rows["sk-test-roundtrip-disabled"].Notes != "paused" {
		t.Fatalf("disabled key did not round-trip: %#v", rows["sk-test-roundtrip-disabled"])
	}
	if rows["sk-test-roundtrip-active"].RequestCount != 0 {
		t.Fatalf("keys-only round-trip copied request count: %#v", rows["sk-test-roundtrip-active"])
	}
}
