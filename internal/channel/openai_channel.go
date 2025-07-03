package channel

import (
	"gpt-load/internal/models"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/datatypes"
)

type OpenAIChannel struct {
	BaseChannel
}

func NewOpenAIChannel(upstreams []string, config datatypes.JSONMap) (*OpenAIChannel, error) {
	base, err := newBaseChannelWithUpstreams("openai", upstreams, config)
	if err != nil {
		return nil, err
	}

	return &OpenAIChannel{
		BaseChannel: base,
	}, nil
}

func (ch *OpenAIChannel) Handle(c *gin.Context, apiKey *models.APIKey, group *models.Group) error {
	modifier := func(req *http.Request, key *models.APIKey) {
		req.Header.Set("Authorization", "Bearer "+key.KeyValue)
	}
	return ch.ProcessRequest(c, apiKey, modifier)
}
