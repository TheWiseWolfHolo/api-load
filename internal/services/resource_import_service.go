package services

import (
	"api-load/internal/models"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

func ParseResourceImportInput(text string) ([]ResourceCreateParams, error) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return nil, nil
	}
	if strings.HasPrefix(trimmed, "{") {
		lines := strings.Split(trimmed, "\n")
		result := make([]ResourceCreateParams, 0, len(lines))
		for i, line := range lines {
			if strings.TrimSpace(line) == "" {
				continue
			}
			var row struct {
				Name        string `json:"name"`
				UpstreamURL string `json:"upstream_url"`
				Key         string `json:"key"`
				Enabled     *bool  `json:"enabled"`
				Priority    int    `json:"priority"`
				Weight      int    `json:"weight"`
			}
			if err := json.Unmarshal([]byte(line), &row); err != nil {
				return nil, fmt.Errorf("row %d: invalid JSONL resource: %w", i+1, err)
			}
			item, err := normalizeResourceImport(i+1, ResourceCreateParams{
				Name: row.Name, UpstreamURL: row.UpstreamURL, Key: row.Key, Enabled: row.Enabled, Priority: row.Priority, Weight: row.Weight,
			})
			if err != nil {
				return nil, err
			}
			result = append(result, item)
		}
		return result, nil
	}

	reader := csv.NewReader(strings.NewReader(trimmed))
	reader.FieldsPerRecord = -1
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("invalid CSV resource import: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil
	}
	header := make(map[string]int, len(rows[0]))
	for i, value := range rows[0] {
		header[strings.ToLower(strings.TrimSpace(value))] = i
	}
	keyIndex, hasKey := header["key"]
	if !hasKey {
		tokens := strings.FieldsFunc(trimmed, func(r rune) bool {
			return unicode.IsSpace(r) || r == ',' || r == '，' || r == ';' || r == '；'
		})
		result := make([]ResourceCreateParams, 0, len(tokens))
		for i, token := range tokens {
			item, normalizeErr := normalizeResourceImport(i+1, ResourceCreateParams{Key: token})
			if normalizeErr != nil {
				return nil, normalizeErr
			}
			result = append(result, item)
		}
		return result, nil
	}
	if len(rows) < 2 {
		return nil, fmt.Errorf("CSV resource import requires at least one data row")
	}
	urlIndex := csvHeaderIndex(header, "upstream_url")
	nameIndex := csvHeaderIndex(header, "name")
	enabledIndex := csvHeaderIndex(header, "enabled")
	priorityIndex := csvHeaderIndex(header, "priority")
	weightIndex := csvHeaderIndex(header, "weight")
	result := make([]ResourceCreateParams, 0, len(rows)-1)
	for i, row := range rows[1:] {
		rowNumber := i + 2
		var enabled *bool
		if raw := csvValue(row, enabledIndex); raw != "" {
			parsed, err := strconv.ParseBool(raw)
			if err != nil {
				return nil, fmt.Errorf("row %d: enabled must be true or false", rowNumber)
			}
			enabled = &parsed
		}
		priority, err := parseOptionalCSVInt(csvValue(row, priorityIndex), rowNumber, "priority")
		if err != nil {
			return nil, err
		}
		weight, err := parseOptionalCSVInt(csvValue(row, weightIndex), rowNumber, "weight")
		if err != nil {
			return nil, err
		}
		item, err := normalizeResourceImport(rowNumber, ResourceCreateParams{
			Name: csvValue(row, nameIndex), UpstreamURL: csvValue(row, urlIndex), Key: csvValue(row, keyIndex),
			Enabled: enabled, Priority: priority, Weight: weight,
		})
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, nil
}

func normalizeResourceImport(rowNumber int, item ResourceCreateParams) (ResourceCreateParams, error) {
	item.Name = strings.TrimSpace(item.Name)
	item.UpstreamURL = strings.TrimSpace(item.UpstreamURL)
	item.Key = strings.TrimSpace(item.Key)
	if item.Key == "" {
		return ResourceCreateParams{}, fmt.Errorf("row %d: key is required", rowNumber)
	}
	if item.Enabled == nil {
		item.Enabled = models.Bool(true)
	}
	if item.Priority == 0 {
		item.Priority = models.DefaultCredentialPriority
	}
	if item.Weight == 0 {
		item.Weight = models.DefaultCredentialWeight
	}
	if item.Priority < 1 || item.Priority > 1000 || item.Weight < 1 || item.Weight > 1000 {
		return ResourceCreateParams{}, fmt.Errorf("row %d: priority and weight must be between 1 and 1000", rowNumber)
	}
	return item, nil
}
