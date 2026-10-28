package modules

import (
	"gpt-load/internal/channel/spec"
	"gpt-load/internal/execution"
	"gpt-load/internal/protocol"
)

// NativeAPI 复用既有多协议执行器，所有路由保留原生格式。
func NativeAPI() spec.Module {
	m := GPTLoad()
	m.Definition.ID = spec.NativeAPI
	m.Definition.Name = "Native API"
	m.Definition.Mark = "NP"
	m.Definition.Icon = "compatible"
	m.Definition.SearchTerms = []string{"native", "passthrough"}
	m.Definition.Description = "Native protocols; upstream support must be verified"
	m.Definition.Routes = append(m.Definition.Routes, spec.NewRoute(protocol.CohereEmbeddings, execution.OperationEmbeddingsCreate, execution.RouteNative))
	return m
}
