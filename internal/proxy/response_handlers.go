package proxy

import (
	"bufio"
	"net/http"

	"io"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

func (ps *ProxyServer) handleStreamingResponse(c *gin.Context, resp *http.Response) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		logrus.Error("Streaming unsupported by the writer, falling back to normal response")
		ps.handleNormalResponse(c, resp)
		return
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		select {
		case <-c.Request.Context().Done():
			logrus.Debugf("Client disconnected, closing stream.")
			return
		default:
		}

		if _, err := c.Writer.Write(scanner.Bytes()); err != nil {
			logUpstreamError("writing stream to client", err)
			return
		}
		if _, err := c.Writer.Write([]byte("\n\n")); err != nil {
			logUpstreamError("writing stream newline to client", err)
			return
		}
		flusher.Flush()
	}

	if err := scanner.Err(); err != nil {
		logUpstreamError("reading from upstream scanner", err)
	}
}

func (ps *ProxyServer) handleNormalResponse(c *gin.Context, resp *http.Response) {
	if _, err := io.Copy(c.Writer, resp.Body); err != nil {
		logUpstreamError("copying response body", err)
	}
}
