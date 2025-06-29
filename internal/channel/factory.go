package channel

import (
	"fmt"
	"gpt-load/internal/models"
)

// GetChannel returns a channel proxy based on the group's channel type.
func GetChannel(group *models.Group) (ChannelProxy, error) {
	switch group.ChannelType {
	case "openai":
		return NewOpenAIChannel(group)
	case "gemini":
		return NewGeminiChannel(group)
	default:
		return nil, fmt.Errorf("unsupported channel type: %s", group.ChannelType)
	}
}