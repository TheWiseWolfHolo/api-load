package utils

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

type secretPattern struct {
	name    string
	pattern *regexp.Regexp
	allow   func(string) bool
}

func TestSEC003RepositoryFixturesUseDummySecretsOnly(t *testing.T) {
	root := repositoryRoot(t)
	patterns := []secretPattern{
		{
			name:    "openai-like-key",
			pattern: regexp.MustCompile(`sk-(?:proj-|live-|ant-|test-|your-|[A-Za-z0-9]{20,})[A-Za-z0-9_-]*`),
			allow: func(match string) bool {
				return strings.HasPrefix(match, "sk-test-") ||
					strings.HasPrefix(match, "sk-your-") ||
					match == "sk-123456" ||
					match == "sk-ant-test" ||
					strings.HasPrefix(match, "sk-ant-api03-test-") ||
					strings.HasPrefix(match, "sk-ant-api03-your-")
			},
		},
		{
			name:    "gemini-like-key",
			pattern: regexp.MustCompile(`AIza[0-9A-Za-z_-]{16,}`),
			allow: func(match string) bool {
				return strings.HasPrefix(match, "AIzaSyDummy")
			},
		},
		{
			name:    "credentialed-proxy-url",
			pattern: regexp.MustCompile(`[a-zA-Z][a-zA-Z0-9+.-]*://[^/\s:@]+:[^@\s/]+@[^)\s"'` + "`" + `]+`),
			allow: func(match string) bool {
				return strings.Contains(match, "example.invalid") ||
					strings.Contains(match, "host:port") ||
					strings.Contains(match, "host:8080") ||
					strings.Contains(match, "dummy-user:dummy-pass@") ||
					strings.Contains(match, "user:password@") ||
					strings.Contains(match, "postgres:123456@postgres")
			},
		},
	}

	var findings []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if shouldSkipSecretScanDir(entry.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !shouldScanSecretFile(path) {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		text := string(content)
		for _, pattern := range patterns {
			for _, match := range pattern.pattern.FindAllString(text, -1) {
				if !pattern.allow(match) {
					rel, _ := filepath.Rel(root, path)
					findings = append(findings, pattern.name+" in "+rel+": "+MaskAPIKey(match))
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("scan repository fixtures: %v", err)
	}
	if len(findings) > 0 {
		t.Fatalf("found non-dummy secret-looking fixtures:\n%s", strings.Join(findings, "\n"))
	}
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("repository root with go.mod not found")
		}
		dir = parent
	}
}

func shouldSkipSecretScanDir(name string) bool {
	switch name {
	case ".git", "node_modules", "dist", ".vite", "data", "tmp":
		return true
	default:
		return false
	}
}

func shouldScanSecretFile(path string) bool {
	switch filepath.Ext(path) {
	case ".go", ".ts", ".vue", ".md", ".yml", ".yaml", ".json", ".env", ".example":
		return true
	default:
		return false
	}
}
