package channel

import (
	"gpt-load/internal/models"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/datatypes"
)

type GeminiChannel struct {
	BaseChannel
}

func NewGeminiChannel(upstreams []string, config datatypes.JSONMap) (*GeminiChannel, error) {
	base, err := newBaseChannelWithUpstreams("gemini", upstreams, config)
	if err != nil {
		return nil, err
	}

	return &GeminiChannel{
		BaseChannel: base,
	}, nil
}

func (ch *GeminiChannel) Handle(c *gin.Context, apiKey *models.APIKey, group *models.Group) error {
	modifier := func(req *http.Request, key *models.APIKey) {
		q := req.URL.Query()
		q.Set("key", key.KeyValue)
		req.URL.RawQuery = q.Encode()
	}
	return ch.ProcessRequest(c, apiKey, modifier)
}
