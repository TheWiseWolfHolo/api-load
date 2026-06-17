package proxy

import (
	"errors"
	"net/url"
	"strings"
)

const (
	ProxyModeInherit = "inherit"
	ProxyModeDirect  = "direct"
	ProxyModeFixed   = "fixed"
	ProxyModePool    = "pool"
)

var ErrProxyItemDisabled = errors.New("proxy item disabled")

type ProxyPolicy struct {
	Mode string `json:"mode"`
	URL  string `json:"url,omitempty"`
}

type ProxyPolicyInputs struct {
	Key    *ProxyPolicy
	Group  *ProxyPolicy
	System *ProxyPolicy
	Env    *ProxyPolicy
}

type ResolvedProxyPolicy struct {
	Mode      string `json:"mode"`
	URL       string `json:"-"`
	MaskedURL string `json:"masked_url,omitempty"`
}

type ProxyPoolItem struct {
	ID      string `json:"id"`
	URL     string `json:"url"`
	Enabled bool   `json:"enabled"`
}

type ProxyPolicyResolver struct{}

func NewProxyPolicyResolver() *ProxyPolicyResolver {
	return &ProxyPolicyResolver{}
}

func (r *ProxyPolicyResolver) Resolve(inputs ProxyPolicyInputs) (ResolvedProxyPolicy, error) {
	for _, policy := range []*ProxyPolicy{inputs.Key, inputs.Group, inputs.System, inputs.Env} {
		if policy == nil || policy.Mode == "" || policy.Mode == ProxyModeInherit {
			continue
		}
		if policy.Mode == ProxyModeDirect {
			return ResolvedProxyPolicy{Mode: ProxyModeDirect}, nil
		}
		if policy.Mode == ProxyModeFixed {
			return ResolvedProxyPolicy{Mode: ProxyModeFixed, URL: policy.URL, MaskedURL: MaskProxyURL(policy.URL)}, nil
		}
		return ResolvedProxyPolicy{Mode: policy.Mode, URL: policy.URL, MaskedURL: MaskProxyURL(policy.URL)}, nil
	}
	return ResolvedProxyPolicy{Mode: ProxyModeInherit}, nil
}

func (r *ProxyPolicyResolver) ResolveFixedProxyItem(item ProxyPoolItem) (ResolvedProxyPolicy, error) {
	if !item.Enabled {
		return ResolvedProxyPolicy{}, ErrProxyItemDisabled
	}
	return ResolvedProxyPolicy{Mode: ProxyModeFixed, URL: item.URL, MaskedURL: MaskProxyURL(item.URL)}, nil
}

func MaskProxyURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.User == nil {
		return rawURL
	}
	var b strings.Builder
	if parsed.Scheme != "" {
		b.WriteString(parsed.Scheme)
		b.WriteString("://")
	}
	b.WriteString("***@")
	b.WriteString(parsed.Host)
	b.WriteString(parsed.EscapedPath())
	if parsed.RawQuery != "" {
		b.WriteString("?")
		b.WriteString(parsed.RawQuery)
	}
	return b.String()
}
