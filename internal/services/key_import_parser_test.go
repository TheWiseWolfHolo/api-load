package services

import (
	"strings"
	"testing"

	"api-load/internal/models"
)

func TestIMP001LegacyTextImportRemainsCompatible(t *testing.T) {
	records, err := ParseKeyImportInput(" sk-test-a\nsk-test-b, sk-test-c\tsk-test-d ")
	if err != nil {
		t.Fatalf("parse legacy text: %v", err)
	}

	want := []string{"sk-test-a", "sk-test-b", "sk-test-c", "sk-test-d"}
	if len(records) != len(want) {
		t.Fatalf("expected %d records, got %d", len(want), len(records))
	}
	for i, record := range records {
		if record.Key != want[i] {
			t.Fatalf("record %d key: expected %q, got %q", i, want[i], record.Key)
		}
		if record.Notes != "" {
			t.Fatalf("record %d notes: expected empty, got %q", i, record.Notes)
		}
		if record.Status != models.KeyStatusActive {
			t.Fatalf("record %d status: expected active, got %q", i, record.Status)
		}
	}
}

func TestIMP002JSONArrayOfStringsRemainsCompatible(t *testing.T) {
	records, err := ParseKeyImportInput(`["sk-test-a","sk-test-b"]`)
	if err != nil {
		t.Fatalf("parse JSON string array: %v", err)
	}

	if len(records) != 2 || records[0].Key != "sk-test-a" || records[1].Key != "sk-test-b" {
		t.Fatalf("unexpected records: %#v", records)
	}
	for _, record := range records {
		if record.Status != models.KeyStatusActive || record.Notes != "" {
			t.Fatalf("expected active record with empty notes, got %#v", record)
		}
	}
}

func TestIMP003JSONLImportSupportsNotesAndStatus(t *testing.T) {
	input := strings.Join([]string{
		`{"key":"sk-test-a","notes":"primary","status":"active"}`,
		`{"key":"sk-test-b","notes":"paused","status":"disabled"}`,
		`{"key":"sk-test-c","notes":"bad","status":"invalid"}`,
	}, "\n")

	records, err := ParseKeyImportInput(input)
	if err != nil {
		t.Fatalf("parse JSONL: %v", err)
	}

	if got := records[1]; got.Key != "sk-test-b" || got.Notes != "paused" || got.Status != models.KeyStatusActive || models.CredentialEnabled(got.Enabled) {
		t.Fatalf("unexpected second record: %#v", got)
	}
}

func TestIMP003JSONLUnknownStatusMasksKeyInError(t *testing.T) {
	_, err := ParseKeyImportInput(`{"key":"sk-test-secret-value","status":"paused"}`)
	if err == nil {
		t.Fatal("expected error for unknown status")
	}
	if !strings.Contains(err.Error(), "row 1") {
		t.Fatalf("expected row number in error, got %q", err.Error())
	}
	if strings.Contains(err.Error(), "sk-test-secret-value") {
		t.Fatalf("error exposed raw key: %q", err.Error())
	}
}

func TestIMP004CSVImportSupportsKeyNotesAndStatusHeaders(t *testing.T) {
	input := "KEY,Notes,Status\nsk-test-a,primary,active\nsk-test-b,paused,disabled\n"

	records, err := ParseKeyImportInput(input)
	if err != nil {
		t.Fatalf("parse CSV: %v", err)
	}

	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}
	if got := records[1]; got.Key != "sk-test-b" || got.Notes != "paused" || got.Status != models.KeyStatusActive || models.CredentialEnabled(got.Enabled) {
		t.Fatalf("unexpected second record: %#v", got)
	}
}

func TestIMP004CSVRejectsRowsWithoutKeyAndLongNotes(t *testing.T) {
	_, err := ParseKeyImportInput("key,notes,status\n,\"missing\",active\n")
	if err == nil || !strings.Contains(err.Error(), "row 2") {
		t.Fatalf("expected row 2 missing key error, got %v", err)
	}

	longNotes := strings.Repeat("a", 256)
	_, err = ParseKeyImportInput("key,notes,status\nsk-test-a," + longNotes + ",active\n")
	if err == nil || !strings.Contains(err.Error(), "notes") {
		t.Fatalf("expected long notes error, got %v", err)
	}
}
