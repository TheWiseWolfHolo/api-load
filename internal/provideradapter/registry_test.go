package provideradapter

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"gpt-load/internal/channel"
	"gpt-load/internal/execution"
	"gpt-load/internal/protocol"
)

type recordingAdapter struct {
	unaryCalls   int
	streamCalls  int
	rejectRoutes bool
	reconciled   []RuntimeTarget
}

func (adapter *recordingAdapter) ValidateRouteCapability(
	_ channel.ProviderKind,
	route channel.RouteDescriptor,
) error {
	if adapter.rejectRoutes || route.ClientProtocol == protocol.TransparentHTTP {
		return fmt.Errorf("route is not implemented")
	}
	return nil
}

func (adapter *recordingAdapter) Reconcile(targets []RuntimeTarget) error {
	adapter.reconciled = targets
	return nil
}

func TestRegistryLeavesTransparentHTTPExecutionAndRuntimeToGateway(t *testing.T) {
	t.Parallel()
	adapter := &recordingAdapter{}
	registry, err := NewRegistry(channel.NewRegistry(), completeBindings(adapter, &recordingAdapter{}))
	if err != nil {
		t.Fatal(err)
	}
	spec := execution.AttemptSpec{ChannelID: string(channel.TransparentHTTP), ClientProtocol: protocol.TransparentHTTP,
		Operation: execution.OperationTransparentRequest, RouteMode: execution.RouteNative}
	result := registry.Execute(t.Context(), spec)
	stream := registry.ExecuteStream(t.Context(), spec, func(execution.StreamEvent) error { return nil })
	if result.Error == nil || result.Error.Code != "gateway_owned_route" || stream.Error == nil || stream.Error.Code != "gateway_owned_route" || adapter.unaryCalls != 0 || adapter.streamCalls != 0 {
		t.Fatalf("transparent request reached SDK: unary=%+v stream=%+v adapter=%+v", result, stream, adapter)
	}
	err = registry.ReconcileTargets([]RuntimeTarget{
		{Target: channel.ResolvedTarget{ChannelID: channel.TransparentHTTP, ProviderKind: channel.ProviderOpenAICompatible}},
		{Target: channel.ResolvedTarget{ChannelID: channel.OpenAI, ProviderKind: channel.ProviderOpenAI}},
	})
	if err != nil || len(adapter.reconciled) != 1 || adapter.reconciled[0].Target.ChannelID != channel.OpenAI {
		t.Fatalf("reconciliation = %+v, err = %v", adapter.reconciled, err)
	}
}

func (adapter *recordingAdapter) Execute(context.Context, execution.AttemptSpec) execution.AttemptResult {
	adapter.unaryCalls++
	return execution.AttemptResult{DispatchState: execution.DispatchMaybeSent, ResponseStarted: true, StatusCode: 200}
}

func (adapter *recordingAdapter) ExecuteStream(
	_ context.Context,
	_ execution.AttemptSpec,
	_ execution.StreamSink,
) execution.StreamResult {
	adapter.streamCalls++
	return execution.StreamResult{DispatchState: execution.DispatchMaybeSent, ResponseStarted: true, StatusCode: 200}
}

func TestRegistryDispatchesOnlyByCompiledChannelProviderBinding(t *testing.T) {
	t.Parallel()

	channels := channel.NewRegistry()
	bifrost := &recordingAdapter{}
	codex := &recordingAdapter{}
	registry, err := NewRegistry(channels, completeBindings(bifrost, codex))
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	registry.Execute(t.Context(), execution.AttemptSpec{
		ChannelID:      string(channel.Codex),
		ClientProtocol: "openai-responses",
		Operation:      execution.OperationResponsesCreate,
		RouteMode:      execution.RouteNative,
		TargetConfig:   json.RawMessage(`{"base_url":"https://chatgpt.com/backend-api/codex"}`),
	})
	registry.ExecuteStream(t.Context(), execution.AttemptSpec{
		ChannelID:      string(channel.OpenAI),
		ClientProtocol: "openai-completions",
		Operation:      execution.OperationChatCompletion,
		RouteMode:      execution.RouteNative,
	}, func(execution.StreamEvent) error { return nil })

	if codex.unaryCalls != 1 || codex.streamCalls != 0 || bifrost.unaryCalls != 0 || bifrost.streamCalls != 1 {
		t.Fatalf("bifrost=%+v codex=%+v", bifrost, codex)
	}
}

func TestRegistryFailsClosedForMissingBindingAndUndeclaredRoute(t *testing.T) {
	t.Parallel()

	channels := channel.NewRegistry()
	if _, err := NewRegistry(channels, []Binding{{ProviderKind: channel.ProviderCodex, Adapter: &recordingAdapter{}}}); err == nil {
		t.Fatal("NewRegistry() accepted incomplete adapter bindings")
	}

	registry, err := NewRegistry(channels, completeBindings(&recordingAdapter{}, &recordingAdapter{}))
	if err != nil {
		t.Fatal(err)
	}
	result := registry.Execute(t.Context(), execution.AttemptSpec{
		ChannelID:      string(channel.Codex),
		ClientProtocol: "openai-responses",
		Operation:      execution.OperationListModels,
		RouteMode:      execution.RouteNative,
		TargetConfig:   json.RawMessage(`{"base_url":"https://chatgpt.com/backend-api/codex"}`),
	})
	if result.DispatchState != execution.DispatchNotSent || result.Error == nil ||
		result.Error.Kind != execution.ErrorKindInvalidRequest ||
		result.Error.OriginHint != execution.ErrorOriginInternal ||
		result.Error.ScopeHint != execution.ErrorScopeGroup ||
		result.Error.Code != "undeclared_channel_route" {
		t.Fatalf("result = %#v", result)
	}
}

func TestRegistryRejectsStoreDowngradeNotDeclaredByChannel(t *testing.T) {
	t.Parallel()

	bifrost := &recordingAdapter{}
	registry, err := NewRegistry(channel.NewRegistry(), completeBindings(bifrost, &recordingAdapter{}))
	if err != nil {
		t.Fatal(err)
	}
	result := registry.Execute(t.Context(), execution.AttemptSpec{
		ChannelID:                string(channel.NewAPI),
		ClientProtocol:           "openai-responses",
		Operation:                execution.OperationResponsesCreate,
		RouteMode:                execution.RouteNative,
		ResponsesStoreDowngraded: true,
		TargetConfig:             json.RawMessage(`{"base_url":"https://newapi.example"}`),
	})
	if bifrost.unaryCalls != 0 || result.Error == nil ||
		result.Error.Code != "undeclared_responses_store_downgrade" {
		t.Fatalf("result/adapter = %#v/%+v", result, bifrost)
	}
}

func TestRegistryRequiresAdaptersToValidateDeclaredCapabilities(t *testing.T) {
	t.Parallel()

	channels := channel.NewRegistry()
	if _, err := NewRegistry(channels, completeBindings(&executorOnlyAdapter{}, &executorOnlyAdapter{})); err == nil {
		t.Fatal("NewRegistry() accepted an adapter without a capability contract")
	}
	if _, err := NewRegistry(channels, completeBindings(&recordingAdapter{}, &recordingAdapter{rejectRoutes: true})); err == nil {
		t.Fatal("NewRegistry() accepted a channel route rejected by its adapter")
	}
}

type executorOnlyAdapter struct{}

func (*executorOnlyAdapter) Execute(context.Context, execution.AttemptSpec) execution.AttemptResult {
	return execution.AttemptResult{}
}

func (*executorOnlyAdapter) ExecuteStream(context.Context, execution.AttemptSpec, execution.StreamSink) execution.StreamResult {
	return execution.StreamResult{}
}

func completeBindings(bifrost execution.Executor, codex execution.Executor) []Binding {
	return []Binding{
		{ProviderKind: channel.ProviderOpenAI, Adapter: bifrost},
		{ProviderKind: channel.ProviderAnthropic, Adapter: bifrost},
		{ProviderKind: channel.ProviderGemini, Adapter: bifrost},
		{ProviderKind: channel.ProviderMultiProtocolGateway, Adapter: bifrost},
		{ProviderKind: channel.ProviderOpenAICompatible, Adapter: bifrost},
		{ProviderKind: channel.ProviderCline, Adapter: bifrost},
		{ProviderKind: channel.ProviderAzureOpenAI, Adapter: bifrost},
		{ProviderKind: channel.ProviderAWSBedrock, Adapter: bifrost},
		{ProviderKind: channel.ProviderGoogleVertex, Adapter: bifrost},
		{ProviderKind: channel.ProviderDeepSeek, Adapter: bifrost},
		{ProviderKind: channel.ProviderOpenRouter, Adapter: bifrost},
		{ProviderKind: channel.ProviderJev, Adapter: bifrost},
		{ProviderKind: channel.ProviderGroq, Adapter: bifrost},
		{ProviderKind: channel.ProviderXAI, Adapter: bifrost},
		{ProviderKind: channel.ProviderCodex, Adapter: codex},
		{ProviderKind: channel.ProviderClaude, Adapter: codex},
		{ProviderKind: channel.ProviderAntigravity, Adapter: codex},
		{ProviderKind: channel.ProviderGrok, Adapter: codex},
	}
}
