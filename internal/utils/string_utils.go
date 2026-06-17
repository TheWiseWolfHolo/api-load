package utils

import (
	"fmt"
	"net/url"
	"strings"
)

// MaskAPIKey masks an API key for safe logging.
func MaskAPIKey(key string) string {
	length := len(key)
	if length == 0 {
		return ""
	}
	if length <= 2 {
		return strings.Repeat("*", length)
	}
	if length <= 8 {
		return fmt.Sprintf("%s****%s", key[:1], key[length-1:])
	}
	return fmt.Sprintf("%s****%s", key[:4], key[length-4:])
}

// MaskURLCredentials removes username and password material from URLs before logging.
func MaskURLCredentials(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return maskRawURLCredentials(rawURL)
	}
	if parsed.User == nil {
		return rawURL
	}
	parsed.User = url.User("***")
	return parsed.String()
}

func maskRawURLCredentials(rawURL string) string {
	schemeIndex := strings.Index(rawURL, "://")
	if schemeIndex < 0 {
		return rawURL
	}
	authorityStart := schemeIndex + len("://")
	atIndex := strings.Index(rawURL[authorityStart:], "@")
	if atIndex < 0 {
		return rawURL
	}
	return rawURL[:authorityStart] + "***@" + rawURL[authorityStart+atIndex+1:]
}

// TruncateString shortens a string to a maximum length.
func TruncateString(s string, maxLength int) string {
	if len(s) > maxLength {
		return s[:maxLength]
	}
	return s
}

// SplitAndTrim splits a string by a separator
func SplitAndTrim(s string, sep string) []string {
	if s == "" {
		return []string{}
	}

	parts := strings.Split(s, sep)
	result := make([]string, 0, len(parts))

	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}

	return result
}

// StringToSet converts a separator-delimited string into a set
func StringToSet(s string, sep string) map[string]struct{} {
	parts := SplitAndTrim(s, sep)
	if len(parts) == 0 {
		return nil
	}

	set := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		set[part] = struct{}{}
	}
	return set
}
