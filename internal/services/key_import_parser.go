package services

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"gpt-load/internal/models"
	"gpt-load/internal/utils"
	"regexp"
	"strings"
	"unicode/utf8"
)

type KeyImportRecord struct {
	Key    string
	Notes  string
	Status string
}

func ParseKeyImportInput(text string) ([]KeyImportRecord, error) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return nil, nil
	}

	if strings.HasPrefix(trimmed, "[") {
		var keys []string
		if err := json.Unmarshal([]byte(trimmed), &keys); err != nil {
			return nil, fmt.Errorf("invalid JSON key import: %w", err)
		}
		records := make([]KeyImportRecord, 0, len(keys))
		for _, key := range keys {
			key = strings.TrimSpace(key)
			if key != "" {
				records = append(records, KeyImportRecord{Key: key, Status: models.KeyStatusActive})
			}
		}
		return records, nil
	}

	if strings.HasPrefix(trimmed, "{") {
		return parseJSONLKeyImport(trimmed)
	}

	if records, ok, err := parseCSVKeyImport(trimmed); ok || err != nil {
		return records, err
	}

	return parseLegacyKeyImport(trimmed), nil
}

func parseLegacyKeyImport(text string) []KeyImportRecord {
	delimiters := regexp.MustCompile(`[\s,;\n\r\t]+`)
	parts := delimiters.Split(strings.TrimSpace(text), -1)
	records := make([]KeyImportRecord, 0, len(parts))
	for _, part := range parts {
		key := strings.TrimSpace(part)
		if key != "" {
			records = append(records, KeyImportRecord{Key: key, Status: models.KeyStatusActive})
		}
	}
	return records
}

func parseJSONLKeyImport(text string) ([]KeyImportRecord, error) {
	lines := strings.Split(text, "\n")
	records := make([]KeyImportRecord, 0, len(lines))
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var row struct {
			Key    string `json:"key"`
			Notes  string `json:"notes"`
			Status string `json:"status"`
		}
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			return nil, fmt.Errorf("row %d: invalid JSONL record: %w", i+1, err)
		}
		record, err := normalizeImportRecord(i+1, row.Key, row.Notes, row.Status)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, nil
}

func parseCSVKeyImport(text string) ([]KeyImportRecord, bool, error) {
	reader := csv.NewReader(strings.NewReader(text))
	reader.FieldsPerRecord = -1
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, true, fmt.Errorf("invalid CSV key import: %w", err)
	}
	if len(rows) == 0 {
		return nil, false, nil
	}

	header := make(map[string]int, len(rows[0]))
	for i, col := range rows[0] {
		header[strings.ToLower(strings.TrimSpace(col))] = i
	}
	keyIndex, ok := header["key"]
	if !ok {
		return nil, false, nil
	}
	notesIndex := header["notes"]
	statusIndex := header["status"]

	records := make([]KeyImportRecord, 0, len(rows)-1)
	for i, row := range rows[1:] {
		rowNumber := i + 2
		key := csvValue(row, keyIndex)
		notes := csvValue(row, notesIndex)
		status := csvValue(row, statusIndex)
		record, err := normalizeImportRecord(rowNumber, key, notes, status)
		if err != nil {
			return nil, true, err
		}
		records = append(records, record)
	}
	return records, true, nil
}

func csvValue(row []string, index int) string {
	if index < 0 || index >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[index])
}

func normalizeImportRecord(rowNumber int, key, notes, status string) (KeyImportRecord, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return KeyImportRecord{}, fmt.Errorf("row %d: key is required", rowNumber)
	}

	notes = strings.TrimSpace(notes)
	if utf8.RuneCountInString(notes) > 255 {
		return KeyImportRecord{}, fmt.Errorf("row %d: notes length must be <= 255 characters for key %s", rowNumber, utils.MaskAPIKey(key))
	}

	status = strings.TrimSpace(status)
	if status == "" {
		status = models.KeyStatusActive
	}
	switch status {
	case models.KeyStatusActive, models.KeyStatusInvalid, models.KeyStatusDisabled:
	default:
		return KeyImportRecord{}, fmt.Errorf("row %d: invalid status %q for key %s", rowNumber, status, utils.MaskAPIKey(key))
	}

	return KeyImportRecord{Key: key, Notes: notes, Status: status}, nil
}
