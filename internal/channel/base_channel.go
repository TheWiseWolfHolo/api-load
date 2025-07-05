package channel

import (
	"fmt"
	"gpt-load/internal/models"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// UpstreamInfo holds the information for a single upstream server, including its weight.
type UpstreamInfo struct {
	URL           *url.URL
	Weight        int
	CurrentWeight int
}

// BaseChannel provides common functionality for channel proxies.
type BaseChannel struct {
	Name         string
	Upstreams    []UpstreamInfo
	HTTPClient   *http.Client
	TestModel    string
	upstreamLock sync.Mutex
}

// RequestModifier is a function that can modify the request before it's sent.
type RequestModifier func(req *http.Request, key *models.APIKey)

// getUpstreamURL selects an upstream URL using a smooth weighted round-robin algorithm.
func (b *BaseChannel) getUpstreamURL() *url.URL {
	b.upstreamLock.Lock()
	defer b.upstreamLock.Unlock()

	if len(b.Upstreams) == 0 {
		return nil
	}
	if len(b.Upstreams) == 1 {
		return b.Upstreams[0].URL
	}

	totalWeight := 0
	var best *UpstreamInfo

	for i := range b.Upstreams {
		up := &b.Upstreams[i]
		totalWeight += up.Weight
		up.CurrentWeight += up.Weight

		if best == nil || up.CurrentWeight > best.CurrentWeight {
			best = up
		}
	}

	if best == nil {
		return b.Upstreams[0].URL // 降级到第一个可用的
	}

	best.CurrentWeight -= totalWeight
	return best.URL
}

// ProcessRequest handles the common logic of processing and forwarding a request.
func (b *BaseChannel) ProcessRequest(c *gin.Context, apiKey *models.APIKey, modifier RequestModifier, ch ChannelProxy) error {
	upstreamURL := b.getUpstreamURL()
	if upstreamURL == nil {
		return fmt.Errorf("no upstream URL configured for channel %s", b.Name)
	}

	director := func(req *http.Request) {
		req.URL.Scheme = upstreamURL.Scheme
		req.URL.Host = upstreamURL.Host
		req.URL.Path = singleJoiningSlash(upstreamURL.Path, req.URL.Path)
		req.Host = upstreamURL.Host

		// Apply the channel-specific modifications
		if modifier != nil {
			modifier(req, apiKey)
		}

		// Remove headers that should not be forwarded
		req.Header.Del("Cookie")
		req.Header.Del("X-Real-Ip")
		req.Header.Del("X-Forwarded-For")
	}

	errorHandler := func(rw http.ResponseWriter, req *http.Request, err error) {
		logrus.WithFields(logrus.Fields{
			"channel": b.Name,
			"key_id":  apiKey.ID,
			"error":   err,
		}).Error("HTTP proxy error")
		rw.WriteHeader(http.StatusBadGateway)
	}

	proxy := &httputil.ReverseProxy{
		Director:     director,
		ErrorHandler: errorHandler,
		Transport:    b.HTTPClient.Transport,
	}

	// Check if the client request is for a streaming endpoint
	if ch.IsStreamingRequest(c) {
		return b.handleStreaming(c, proxy)
	}

	proxy.ServeHTTP(c.Writer, c.Request)
	return nil
}

func (b *BaseChannel) handleStreaming(c *gin.Context, proxy *httputil.ReverseProxy) error {
	var wg sync.WaitGroup
	wg.Add(1)

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("Access-Control-Allow-Origin", "*")

	// Use a pipe to avoid buffering the entire response
	pr, pw := io.Pipe()
	defer pr.Close()

	req := c.Request.Clone(c.Request.Context())
	req.Body = pr

	// Start the proxy in a goroutine
	go func() {
		defer wg.Done()
		defer pw.Close()
		proxy.ServeHTTP(c.Writer, req)
	}()

	// Copy the original request body to the pipe writer
	_, err := io.Copy(pw, c.Request.Body)
	if err != nil {
		logrus.Errorf("Error copying request body to pipe: %v", err)
		wg.Wait() // Wait for the goroutine to finish even if copy fails
		return err
	}

	// Wait for the proxy to finish
	wg.Wait()

	return nil
}

// singleJoiningSlash joins two URL paths with a single slash.
func singleJoiningSlash(a, b string) string {
	aslash := strings.HasSuffix(a, "/")
	bslash := strings.HasPrefix(b, "/")
	switch {
	case aslash && bslash:
		return a + b[1:]
	case !aslash && !bslash:
		return a + "/" + b
	}
	return a + b
}
