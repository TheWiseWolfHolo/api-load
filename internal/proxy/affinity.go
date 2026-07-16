package proxy

import (
	"api-load/internal/encryption"
	"encoding/json"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
)

const affinityHeader = "X-Api-Load-Affinity"

var userAgentVersionPattern = regexp.MustCompile(`\b[vV]?\d+(?:\.\d+){1,3}\b`)

type requestAffinity struct {
	Hash   string
	Source string
}

func deriveRequestAffinity(c *gin.Context, body []byte, proxyKey string, encryptionSvc encryption.Service) requestAffinity {
	if encryptionSvc == nil {
		return requestAffinity{}
	}
	if c != nil {
		if explicit := strings.TrimSpace(c.GetHeader(affinityHeader)); explicit != "" {
			if len(explicit) > 512 {
				explicit = explicit[:512]
			}
			return hashedAffinity(encryptionSvc, "explicit", explicit)
		}
	}

	var payload any
	parsed := json.Unmarshal(body, &payload) == nil
	if parsed {
		if session := metadataSessionID(payload); session != "" {
			return hashedAffinity(encryptionSvc, "metadata", session)
		}
		anchors := make([]string, 0, 4)
		collectCacheControlAnchors(payload, &anchors)
		if len(anchors) > 0 {
			return hashedAffinity(encryptionSvc, "cache_control", strings.Join(anchors, "\x1e"))
		}
	}

	stableContent := stableRequestContent(payload)
	if stableContent == "" && len(body) > 0 {
		stableContent = string(body)
	}
	if stableContent == "" {
		return requestAffinity{}
	}
	clientIP, userAgent := "", ""
	if c != nil && c.Request != nil {
		clientIP = c.ClientIP()
		userAgent = normalizeAffinityUserAgent(c.Request.UserAgent())
	}
	identity := strings.Join([]string{clientIP, userAgent, encryptionSvc.Hash(proxyKey), stableContent}, "\x1f")
	return hashedAffinity(encryptionSvc, "request_digest", identity)
}

func hashedAffinity(encryptionSvc encryption.Service, source, value string) requestAffinity {
	return requestAffinity{Hash: encryptionSvc.Hash(source + "\x00" + value), Source: source}
}

func metadataSessionID(payload any) string {
	root, ok := payload.(map[string]any)
	if !ok {
		return ""
	}
	metadata, ok := root["metadata"].(map[string]any)
	if !ok {
		return ""
	}
	if session, ok := metadata["session_id"].(string); ok && strings.TrimSpace(session) != "" {
		return strings.TrimSpace(session)
	}
	userID, _ := metadata["user_id"].(string)
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return ""
	}
	var nested map[string]any
	if json.Unmarshal([]byte(userID), &nested) == nil {
		if session, ok := nested["session_id"].(string); ok && strings.TrimSpace(session) != "" {
			return strings.TrimSpace(session)
		}
	}
	return userID
}

func collectCacheControlAnchors(value any, anchors *[]string) {
	switch node := value.(type) {
	case map[string]any:
		if cacheControl, ok := node["cache_control"].(map[string]any); ok && cacheControl["type"] == "ephemeral" {
			if encoded, err := json.Marshal(node); err == nil {
				*anchors = append(*anchors, string(encoded))
			}
			return
		}
		for _, child := range node {
			collectCacheControlAnchors(child, anchors)
		}
	case []any:
		for _, child := range node {
			collectCacheControlAnchors(child, anchors)
		}
	}
}

func stableRequestContent(payload any) string {
	root, ok := payload.(map[string]any)
	if !ok {
		return ""
	}
	parts := make([]string, 0, 2)
	if system, ok := root["system"]; ok {
		if encoded, err := json.Marshal(system); err == nil {
			parts = append(parts, string(encoded))
		}
	}
	for _, field := range []string{"messages", "input"} {
		items, ok := root[field].([]any)
		if !ok {
			continue
		}
		for _, item := range items {
			message, ok := item.(map[string]any)
			if !ok {
				continue
			}
			role, _ := message["role"].(string)
			if role != "user" {
				continue
			}
			if encoded, err := json.Marshal(message["content"]); err == nil {
				parts = append(parts, string(encoded))
			}
			break
		}
		if len(parts) > 0 {
			break
		}
	}
	return strings.Join(parts, "\x1e")
}

func normalizeAffinityUserAgent(userAgent string) string {
	return strings.TrimSpace(userAgentVersionPattern.ReplaceAllString(userAgent, "*"))
}
