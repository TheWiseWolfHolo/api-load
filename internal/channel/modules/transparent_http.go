package modules

import (
	"gpt-load/internal/channel/spec"
	"gpt-load/internal/execution"
	"gpt-load/internal/protocol"
)

// TransparentHTTP has no model catalog; its administrator-owned target lives in group overrides.
func TransparentHTTP() spec.Module {
	m := OpenAICompatible()
	m.Definition.ID = spec.TransparentHTTP
	m.Definition.Name = "Transparent HTTP"
	m.Definition.Mark = "HTTP"
	m.Definition.Icon = ""
	m.Definition.Description = "Explicitly scoped HTTP forwarding without model translation"
	m.Definition.SearchTerms = []string{"transparent", "http", "proxy"}
	m.Definition.Params = nil
	m.Definition.Provider.EndpointPolicy = spec.EndpointNone
	m.Definition.Routes = []spec.Route{spec.NewRoute(protocol.TransparentHTTP, execution.OperationTransparentRequest, execution.RouteNative)}
	return m
}
