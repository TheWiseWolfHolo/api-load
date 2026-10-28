package modules

import (
	"gpt-load/internal/channel/spec"
	"gpt-load/internal/execution"
	"gpt-load/internal/protocol"
)

func Soniox() spec.Module {
	return spec.Module{
		Definition: spec.Definition{
			ID:          spec.Soniox,
			Name:        "Soniox",
			Mark:        "SO",
			Icon:        "soniox",
			SearchTerms: []string{"soniox", "speech", "transcription"},
			Description: "Soniox native asynchronous transcription and files",
			Connection: spec.Connection{
				Type:            spec.ConnectionAPIKey,
				CredentialInput: "batch_text",
			},
			Params: []spec.Field{{
				Key: "base_url", Label: "Base URL", InputKind: spec.InputURL,
				Normalizer: spec.NormalizeBaseURL,
			}, {Key: "tts_base_url", Label: "TTS Base URL", InputKind: spec.InputURL, Normalizer: spec.NormalizeBaseURL}},
			Credentials: []spec.Field{{
				Key: "api_key", Label: "API Key", InputKind: spec.InputSecret,
				Required: true, Sensitive: true, Normalizer: spec.NormalizeNonEmpty,
			}},
			Provider: spec.ProviderBinding{
				ProviderKind:      spec.ProviderOpenAICompatible,
				CatalogProviderID: "soniox",
				EndpointPolicy:    spec.EndpointFixedWithOverride,
				FixedBaseURL:      "https://api.soniox.com/v1",
			},
			Routes: []spec.Route{
				spec.NewRoute(protocol.NativeServices, execution.OperationResourceCreate, execution.RouteNative),
				spec.NewRoute(protocol.NativeServices, execution.OperationResourceRead, execution.RouteNative),
				spec.NewRoute(protocol.NativeServices, execution.OperationResourceDelete, execution.RouteNative),
			},
		},
	}
}
