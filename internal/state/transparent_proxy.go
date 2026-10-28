package state

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"slices"
	"strings"

	"gpt-load/internal/connection"
)

type TransparentProxyConfig struct {
	Enabled           bool   `json:"enabled"`
	BaseURL           string `json:"base_url"`
	AuthHeader        string `json:"auth_header"`
	AuthPrefix        string `json:"auth_prefix"`
	Strategy          string `json:"strategy"`
	FixedCredentialID uint   `json:"fixed_credential_id"`
}

type TransparentPathRule struct {
	Path  string `json:"path"`
	Match string `json:"match"`
}

type TransparentScope struct {
	GroupIDs []uint                `json:"group_ids"`
	Methods  []string              `json:"methods"`
	Paths    []TransparentPathRule `json:"paths"`
}

func (s *TransparentScope) Clone() *TransparentScope {
	if s == nil {
		return nil
	}
	return &TransparentScope{GroupIDs: slices.Clone(s.GroupIDs), Methods: slices.Clone(s.Methods), Paths: slices.Clone(s.Paths)}
}

func (f FilterSet) AllowsTransparent(groupID uint, method, requestPath string) bool {
	s := f.Transparent
	clean := path.Clean(requestPath)
	if !strings.HasPrefix(requestPath, "/") || strings.ContainsAny(requestPath, "%?#\\\r\n\x00") || (requestPath != clean && requestPath != clean+"/") {
		return false
	}
	if s == nil || len(f.Models) > 0 || !f.AllowsGroup(groupID) || !slices.Contains(s.GroupIDs, groupID) || !slices.Contains(s.Methods, method) {
		return false
	}
	if _, ok := f.Protocols["transparent-http"]; !ok {
		return false
	}
	for _, rule := range s.Paths {
		if requestPath == rule.Path || rule.Match == "subtree" && (rule.Path == "/" || strings.HasPrefix(requestPath, rule.Path+"/")) {
			return true
		}
	}
	return false
}

// NormalizeTransparentScope validates an explicit management/file boundary, with no wildcard defaults.
func NormalizeTransparentScope(input *TransparentScope) (*TransparentScope, error) {
	if input == nil {
		return nil, nil
	}
	result := input.Clone()
	if len(result.GroupIDs) == 0 || len(result.Methods) == 0 || len(result.Paths) == 0 {
		return nil, fmt.Errorf("transparent scope requires groups, methods and paths")
	}
	for _, id := range result.GroupIDs {
		if id == 0 {
			return nil, fmt.Errorf("transparent group ID must be positive")
		}
	}
	slices.Sort(result.GroupIDs)
	result.GroupIDs = slices.Compact(result.GroupIDs)
	for _, method := range result.Methods {
		if !slices.Contains([]string{"GET", "HEAD", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}, method) {
			return nil, fmt.Errorf("invalid transparent method")
		}
	}
	slices.Sort(result.Methods)
	result.Methods = slices.Compact(result.Methods)
	for _, rule := range result.Paths {
		if rule.Match != "exact" && rule.Match != "subtree" {
			return nil, fmt.Errorf("invalid transparent path match")
		}
		if !strings.HasPrefix(rule.Path, "/") || strings.ContainsAny(rule.Path, "%?#\\\r\n\x00") || path.Clean(rule.Path) != rule.Path {
			return nil, fmt.Errorf("transparent paths must be canonical absolute paths")
		}
	}
	slices.SortFunc(result.Paths, func(a, b TransparentPathRule) int {
		if n := strings.Compare(a.Path, b.Path); n != 0 {
			return n
		}
		return strings.Compare(a.Match, b.Match)
	})
	result.Paths = slices.Compact(result.Paths)
	return result, nil
}

func parseTransparentProxy(value any) (TransparentProxyConfig, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return TransparentProxyConfig{}, err
	}
	var result TransparentProxyConfig
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if bytes.Equal(raw, []byte("null")) {
		return result, fmt.Errorf("transparent_proxy must be an object")
	}
	if err = decoder.Decode(&result); err != nil {
		return result, err
	}
	if !result.Enabled {
		return result, nil
	}
	u, err := url.Parse(result.BaseURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.RawPath != "" || strings.ContainsAny(u.Path, "\\\x00\r\n") || strings.Contains(u.Path, "//") || u.Path != "" && u.Path != "/" && path.Clean(u.Path) != strings.TrimSuffix(u.Path, "/") {
		return result, fmt.Errorf("transparent base URL must be an absolute HTTP address without credentials, query or fragment")
	}
	if result.AuthHeader == "" || strings.Trim(result.AuthHeader, "!#$%&'*+-.^_`|~0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ") != "" {
		return result, fmt.Errorf("invalid transparent authentication header")
	}
	if slices.Contains([]string{"host", "content-length", "transfer-encoding", "connection", "proxy-authorization", "proxy-connection", "upgrade", "te", "trailer"}, strings.ToLower(result.AuthHeader)) {
		return result, fmt.Errorf("transport headers cannot carry transparent credentials")
	}
	for _, r := range result.AuthPrefix {
		if r < 32 || r == 127 {
			return result, fmt.Errorf("invalid transparent authentication prefix")
		}
	}
	if result.Strategy != "rotate" && result.Strategy != "fixed" {
		return result, fmt.Errorf("transparent strategy must be rotate or fixed")
	}
	if result.Strategy == "fixed" && result.FixedCredentialID == 0 || result.Strategy == "rotate" && result.FixedCredentialID != 0 {
		return result, fmt.Errorf("fixed credential is required only for the fixed strategy")
	}
	return result, nil
}

func validateTransparentGroup(input CompileInput, group GroupConfig, proxy TransparentProxyConfig) error {
	if !proxy.Enabled {
		return nil
	}
	descriptor, _ := input.ChannelRegistry.Get(group.ChannelID)
	if connection.Normalize(group.ConnectionType) != connection.APIKey || len(descriptor.CredentialFields) != 1 || descriptor.CredentialFields[0].Key != "api_key" {
		return fmt.Errorf("group %d transparent proxy requires a static API-key source", group.ID)
	}
	if proxy.Strategy == "fixed" {
		for _, credential := range input.Credentials {
			if credential.ID == proxy.FixedCredentialID && credential.GroupID == group.ID {
				return nil
			}
		}
		return fmt.Errorf("group %d fixed transparent credential does not belong to this group", group.ID)
	}
	return nil
}
