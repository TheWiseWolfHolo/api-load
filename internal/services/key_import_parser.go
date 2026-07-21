package services

import (
	"api-load/internal/models"
	"api-load/internal/utils"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

type KeyImportRecord struct {
	Key      string
	Notes    string
	Status   string
	Enabled  *bool
	Priority int
	Weight   int
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
				records = append(records, defaultKeyImportRecord(key))
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
			records = append(records, defaultKeyImportRecord(key))
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
			Key      string `json:"key"`
			Notes    string `json:"notes"`
			Status   string `json:"status"`
			Enabled  *bool  `json:"enabled"`
			Priority int    `json:"priority"`
			Weight   int    `json:"weight"`
		}
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			return nil, fmt.Errorf("row %d: invalid JSONL record: %w", i+1, err)
		}
		record, err := normalizeImportRecord(i+1, row.Key, row.Notes, row.Status, row.Enabled, row.Priority, row.Weight)
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
	notesIndex := csvHeaderIndex(header, "notes")
	statusIndex := csvHeaderIndex(header, "status")
	enabledIndex := csvHeaderIndex(header, "enabled")
	priorityIndex := csvHeaderIndex(header, "priority")
	weightIndex := csvHeaderIndex(header, "weight")

	records := make([]KeyImportRecord, 0, len(rows)-1)
	for i, row := range rows[1:] {
		rowNumber := i + 2
		key := csvValue(row, keyIndex)
		notes := csvValue(row, notesIndex)
		status := csvValue(row, statusIndex)
		var enabled *bool
		if raw := csvValue(row, enabledIndex); raw != "" {
			parsed, parseErr := strconv.ParseBool(raw)
			if parseErr != nil {
				return nil, true, fmt.Errorf("row %d: enabled must be true or false", rowNumber)
			}
			enabled = &parsed
		}
		priority, err := parseOptionalCSVInt(csvValue(row, priorityIndex), rowNumber, "priority")
		if err != nil {
			return nil, true, err
		}
		weight, err := parseOptionalCSVInt(csvValue(row, weightIndex), rowNumber, "weight")
		if err != nil {
			return nil, true, err
		}
		record, err := normalizeImportRecord(rowNumber, key, notes, status, enabled, priority, weight)
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

func normalizeImportRecord(rowNumber int, key, notes, status string, enabled *bool, priority, weight int) (KeyImportRecord, error) {
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
	case models.KeyStatusDisabled:
		status = models.KeyStatusActive
		enabled = models.Bool(false)
	case models.KeyStatusActive, models.KeyStatusInvalid:
	default:
		return KeyImportRecord{}, fmt.Errorf("row %d: invalid status %q for key %s", rowNumber, status, utils.MaskAPIKey(key))
	}
	if enabled == nil {
		enabled = models.Bool(true)
	}
	if priority == 0 {
		priority = models.DefaultCredentialPriority
	}
	if weight == 0 {
		weight = models.DefaultCredentialWeight
	}
	if priority < 1 || priority > 1000 {
		return KeyImportRecord{}, fmt.Errorf("row %d: priority must be between 1 and 1000 for key %s", rowNumber, utils.MaskAPIKey(key))
	}
	if weight < 1 || weight > 1000 {
		return KeyImportRecord{}, fmt.Errorf("row %d: weight must be between 1 and 1000 for key %s", rowNumber, utils.MaskAPIKey(key))
	}

	return KeyImportRecord{Key: key, Notes: notes, Status: status, Enabled: enabled, Priority: priority, Weight: weight}, nil
}

func defaultKeyImportRecord(key string) KeyImportRecord {
	return KeyImportRecord{Key: key, Status: models.KeyStatusActive, Enabled: models.Bool(true), Priority: models.DefaultCredentialPriority, Weight: models.DefaultCredentialWeight}
}

func csvHeaderIndex(header map[string]int, name string) int {
	if index, ok := header[name]; ok {
		return index
	}
	return -1
}

func parseOptionalCSVInt(raw string, rowNumber int, field string) (int, error) {
	if raw == "" {
		return 0, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("row %d: %s must be an integer", rowNumber, field)
	}
	return value, nil
}
