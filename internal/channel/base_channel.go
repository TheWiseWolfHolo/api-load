package channel

import (
	"fmt"
	"gpt-load/internal/models"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync/atomic"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/sirupsen/logrus"
)

// BaseChannel provides common functionality for channel proxies.
type BaseChannel struct {
	Name          string
	Upstreams     []*url.URL
	HTTPClient    *http.Client
	roundRobin    uint64
}

// RequestModifier is a function that can modify the request before it's sent.
type RequestModifier func(req *http.Request, key *models.APIKey)

// getUpstreamURL selects an upstream URL using round-robin.
func (b *BaseChannel) getUpstreamURL() *url.URL {
	if len(b.Upstreams) == 0 {
		return nil
	}
	if len(b.Upstreams) == 1 {
		return b.Upstreams[0]
	}
	index := atomic.AddUint64(&b.roundRobin, 1) - 1
	return b.Upstreams[index%uint64(len(b.Upstreams))]
}

// ProcessRequest handles the common logic of processing and forwarding a request.
func (b *BaseChannel) ProcessRequest(c *gin.Context, apiKey *models.APIKey, modifier RequestModifier) error {
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
	if isStreamingRequest(c) {
		return b.handleStreaming(c, proxy)
	}

	proxy.ServeHTTP(c.Writer, c.Request)
	return nil
}

func (b *BaseChannel) handleStreaming(c *gin.Context, proxy *httputil.ReverseProxy) error {
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("Access-Control-Allow-Origin", "*")

	// Use a pipe to avoid buffering the entire response
	pr, pw := io.Pipe()
	defer pr.Close()

	// Create a new request with the pipe reader as the body
	// This is a bit of a hack to get ReverseProxy to stream
	req := c.Request.Clone(c.Request.Context())
	req.Body = pr

	// Start the proxy in a goroutine
	go func() {
		defer pw.Close()
		proxy.ServeHTTP(c.Writer, req)
	}()

	// Copy the original request body to the pipe writer
	_, err := io.Copy(pw, c.Request.Body)
	if err != nil {
		logrus.Errorf("Error copying request body to pipe: %v", err)
		return err
	}

	return nil
}

// isStreamingRequest checks if the request is for a streaming response.
func isStreamingRequest(c *gin.Context) bool {
	// For Gemini, streaming is indicated by the path.
	if strings.Contains(c.Request.URL.Path, ":streamGenerateContent") {
		return true
	}

	// For OpenAI, streaming is indicated by a "stream": true field in the JSON body.
	// We use ShouldBindBodyWith to check the body without consuming it, so it can be read again by the proxy.
	type streamPayload struct {
		Stream bool `json:"stream"`
	}
	var p streamPayload
	if err := c.ShouldBindBodyWith(&p, binding.JSON); err == nil {
		return p.Stream
	}

	return false
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
