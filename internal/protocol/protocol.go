// Package protocol defines protocol identifiers shared by runtime domains.
package protocol

type Protocol string

const NativeServices Protocol = "native-services"
const TransparentHTTP Protocol = "transparent-http"

const (
	OpenAICompletions Protocol = "openai-completions"
	OpenAIResponses   Protocol = "openai-responses"
	OpenAIImages      Protocol = "openai-images"
	OpenAIEmbeddings  Protocol = "openai-embeddings"
	CodexLive         Protocol = "codex-live"
	Rerank            Protocol = "rerank"
	Decisions         Protocol = "decisions"
	Anthropic         Protocol = "anthropic"
	Gemini            Protocol = "gemini"
	GeminiEmbeddings  Protocol = "gemini-embeddings"
	CohereEmbeddings  Protocol = "cohere-embeddings"
	AudioSpeech       Protocol = "audio-speech"
	// Mistral is the native surface for Mistral-only HTTP APIs. Chat and
	// embeddings stay on the OpenAI-compatible protocols.
	Mistral Protocol = "mistral"
)

func (p Protocol) Valid() bool {
	switch p {
	case OpenAICompletions, OpenAIResponses, OpenAIImages, OpenAIEmbeddings, CodexLive, Rerank, Decisions,
		Anthropic, Gemini, GeminiEmbeddings, CohereEmbeddings, AudioSpeech, Mistral, NativeServices, TransparentHTTP:
		return true
	default:
		return false
	}
}

func (p Protocol) DataPlaneEnabled() bool {
	switch p {
	case OpenAICompletions, OpenAIResponses, OpenAIImages, OpenAIEmbeddings, CodexLive, Rerank, Decisions,
		Anthropic, Gemini, GeminiEmbeddings, CohereEmbeddings, AudioSpeech, Mistral, NativeServices, TransparentHTTP:
		return true
	default:
		return false
	}
}

func (p Protocol) SupportsModelOptionalRequests() bool {
	return p == OpenAIResponses || p == NativeServices || p == TransparentHTTP
}

func DataPlaneProtocols() []Protocol {
	return []Protocol{
		OpenAICompletions,
		OpenAIResponses,
		OpenAIImages,
		OpenAIEmbeddings,
		CodexLive,
		Rerank,
		Decisions,
		AudioSpeech,
		Anthropic,
		Gemini,
		GeminiEmbeddings,
		CohereEmbeddings,
		Mistral,
		NativeServices,
		TransparentHTTP,
	}
}
