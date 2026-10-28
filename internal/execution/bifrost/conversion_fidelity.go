package bifrost

import (
	"context"
	"strings"

	"github.com/maximhq/bifrost/core/providers/anthropic"
	"github.com/maximhq/bifrost/core/schemas"

	"gpt-load/internal/channel"
	"gpt-load/internal/dialect"
	"gpt-load/internal/execution"
)

func finishConvertedPreparation(spec execution.AttemptSpec, providerKind channel.ProviderKind, prepared preparedAttempt) (preparedAttempt, *execution.AttemptResult) {
	if spec.RouteMode != execution.RouteConverted ||
		(spec.Operation != execution.OperationChatCompletion && spec.Operation != execution.OperationResponsesCreate) {
		return prepared, nil
	}
	var toolConstraintsValid bool
	var needsToolHistoryCheck bool
	prepared, needsToolHistoryCheck, toolConstraintsValid = prepareConvertedToolConstraints(spec.ClientProtocol, providerKind, spec.UpstreamModel, prepared)
	if !toolConstraintsValid {
		failure := notSentConversionFailure(execution.ErrorCodeCriticalSemanticLoss, "conversion cannot preserve requested tools or tool choice")
		return preparedAttempt{}, &failure
	}
	vertexChat := providerKind == channel.ProviderGoogleVertex && vertexUsesChatFallback(spec.UpstreamModel)
	if providerKind == channel.ProviderAnthropic || providerKind == channel.ProviderGemini ||
		providerKind == channel.ProviderAWSBedrock || (providerKind == channel.ProviderGoogleVertex && !vertexChat) {
		preserveResponsesGlobalInstructions(prepared.responsesRequest)
	}
	chatFallback := providerKind == channel.ProviderOpenAICompatible || providerKind == channel.ProviderCline || providerKind == channel.ProviderGroq ||
		providerKind == channel.ProviderDeepSeek || vertexChat
	if chatFallback {
		if prepared.responsesRequest != nil && prepared.responsesRequest.Params != nil && prepared.responsesRequest.Params.Reasoning != nil && prepared.responsesRequest.Params.Reasoning.MaxTokens != nil && *prepared.responsesRequest.Params.Reasoning.MaxTokens != 0 {
			failure := notSentConversionFailure(execution.ErrorCodeCriticalSemanticLoss, "Chat conversion cannot preserve an exact reasoning token budget; use a native protocol route")
			return preparedAttempt{}, &failure
		}
		dropsTools, newlyAllowed := chatFallbackToolCompatibility(prepared.responsesRequest)
		needsToolHistoryCheck = needsToolHistoryCheck || newlyAllowed
		if dropsTools {
			failure := notSentConversionFailure(execution.ErrorCodeCriticalSemanticLoss, "Chat conversion cannot preserve requested tools or tool choice")
			return preparedAttempt{}, &failure
		}
		if providerKind == channel.ProviderOpenAICompatible && !flattenCompatibleToolOutputs(prepared.responsesRequest) {
			failure := notSentConversionFailure(execution.ErrorCodeCriticalSemanticLoss, "Chat conversion cannot preserve images, files or audio in tool results; use a native protocol route")
			return preparedAttempt{}, &failure
		}
		if providerKind == channel.ProviderDeepSeek && deepSeekConversionDisablesThinking(prepared.responsesRequest) {
			failure := notSentConversionFailure(execution.ErrorCodeCriticalSemanticLoss, "DeepSeek conversion cannot preserve explicit thinking with this tool choice or history")
			return preparedAttempt{}, &failure
		}
	}
	if needsToolHistoryCheck && !convertedTargetPreservesToolHistory(providerKind, spec.UpstreamModel, prepared.responsesRequest) {
		failure := notSentConversionFailure(execution.ErrorCodeCriticalSemanticLoss, "conversion cannot preserve requested tool history")
		return preparedAttempt{}, &failure
	}
	wantSystems := dialect.CountMidConversationSystemMessages(spec.ClientProtocol, spec.Body)
	if wantSystems == 0 {
		return prepared, nil
	}
	preserved := true
	switch providerKind {
	case channel.ProviderGemini, channel.ProviderAWSBedrock:
		// 这些现有转换器只能把中途系统指令前移或变成用户文本。
		preserved = false
	case channel.ProviderGoogleVertex:
		// Vertex 的非 Claude/Gemini 模型使用 OpenAI wire，可以原位保留系统消息。
		preserved = vertexChat
	case channel.ProviderAnthropic:
		// 仅对此输入形状复用 SDK 的 ModelCaps 和实际位置规则，避免复制模型表或近似工具分组。
		ctx := schemas.NewBifrostContext(context.Background(), schemas.NoDeadline)
		defer ctx.Cancel()
		var request *anthropic.AnthropicMessageRequest
		var err error
		if prepared.request != nil {
			request, err = anthropic.ToAnthropicChatRequest(ctx, prepared.request)
		} else {
			request, err = anthropic.ToAnthropicResponsesRequest(ctx, prepared.responsesRequest)
		}
		keptSystems := 0
		if err == nil && request != nil {
			for _, message := range request.Messages {
				if message.Role == anthropic.AnthropicMessageRoleSystem {
					keptSystems++
				}
			}
		}
		preserved = keptSystems == wantSystems
	}
	if !preserved {
		failure := notSentConversionFailure(execution.ErrorCodeCriticalSemanticLoss, "conversion cannot preserve mid-conversation system instructions")
		return preparedAttempt{}, &failure
	}
	return prepared, nil
}

func vertexUsesChatFallback(model string) bool {
	ctx := schemas.NewBifrostContext(context.Background(), schemas.NoDeadline)
	defer ctx.Cancel()
	return !schemas.IsAnthropicModelFamily(ctx, model) && !schemas.IsGeminiModelFamily(ctx, model) &&
		!schemas.IsGemmaModelFamily(ctx, model) && !schemas.IsAllDigitsASCII(model)
}

func preserveResponsesGlobalInstructions(request *schemas.BifrostResponsesRequest) {
	if request == nil || request.Params == nil || request.Params.Instructions == nil || *request.Params.Instructions == "" {
		return
	}
	// 这些 SDK 转换器优先使用 input 内的 system，可能覆盖并存的 instructions。
	// 将全局指令放在输入前端，让同一转换器按顺序处理，并避免重复发送。
	request.Input = append([]schemas.ResponsesMessage{{
		Type:    schemas.Ptr(schemas.ResponsesMessageTypeMessage),
		Role:    schemas.Ptr(schemas.ResponsesInputMessageRoleSystem),
		Content: &schemas.ResponsesMessageContent{ContentStr: request.Params.Instructions},
	}}, request.Input...)
	request.Params.Instructions = nil
}

// flattenCompatibleToolOutputs 只把纯文字结果转为字符串，拒绝非文字内容。

func flattenCompatibleToolOutputs(request *schemas.BifrostResponsesRequest) bool {
	if request == nil {
		return true
	}
	for _, item := range request.Input {
		if item.Type == nil || *item.Type != schemas.ResponsesMessageTypeFunctionCallOutput || item.ResponsesToolMessage == nil || item.ResponsesToolMessage.Output == nil {
			continue
		}
		for _, block := range item.ResponsesToolMessage.Output.ResponsesFunctionToolCallOutputBlocks {
			if block.Text == nil || !isTextContentBlockType(block.Type) {
				return false
			}
		}
	}
	for i := range request.Input {
		item := &request.Input[i]
		if item.Type == nil || *item.Type != schemas.ResponsesMessageTypeFunctionCallOutput || item.ResponsesToolMessage == nil || item.ResponsesToolMessage.Output == nil {
			continue
		}
		output := item.ResponsesToolMessage.Output
		if len(output.ResponsesFunctionToolCallOutputBlocks) == 0 {
			continue
		}
		parts := make([]string, 0, len(output.ResponsesFunctionToolCallOutputBlocks))
		for _, block := range output.ResponsesFunctionToolCallOutputBlocks {
			parts = append(parts, *block.Text)
		}
		flattened := strings.Join(parts, "\n\n")
		output.ResponsesToolCallOutputStr = &flattened
		output.ResponsesFunctionToolCallOutputBlocks = nil
		if item.Content != nil && len(item.Content.ContentBlocks) > 0 {
			item.Content.ContentBlocks = nil
			item.Content.ContentStr = &flattened
		}
	}
	return true
}

func isTextContentBlockType(blockType schemas.ResponsesMessageContentBlockType) bool {
	switch blockType {
	case schemas.ResponsesInputMessageContentBlockTypeText,
		schemas.ResponsesOutputMessageContentTypeText,
		schemas.ResponsesOutputMessageContentTypeSummaryText,
		schemas.ResponsesOutputMessageContentTypeReasoning:
		return true
	}
	return false
}

func deepSeekConversionDisablesThinking(request *schemas.BifrostResponsesRequest) bool {
	if request == nil || request.Params == nil || request.Params.Reasoning == nil {
		return false
	}
	reasoning := request.Params.Reasoning
	enabled := (reasoning.Effort != nil && *reasoning.Effort != "" && *reasoning.Effort != "none") ||
		(reasoning.MaxTokens != nil && *reasoning.MaxTokens != 0)
	if !enabled {
		return false
	}
	chat := request.ToChatRequest()
	// 对齐锁定 SDK 的 requiresDeepSeekThinkingDisabled，仅阻断显式开启思考的冲突输入。
	if choice := chat.Params.ToolChoice; choice != nil {
		var kind schemas.ChatToolChoiceType
		if choice.ChatToolChoiceStr != nil {
			kind = schemas.ChatToolChoiceType(*choice.ChatToolChoiceStr)
		} else if choice.ChatToolChoiceStruct != nil {
			kind = choice.ChatToolChoiceStruct.Type
		}
		switch kind {
		case schemas.ChatToolChoiceTypeRequired, schemas.ChatToolChoiceTypeAny, schemas.ChatToolChoiceTypeFunction,
			schemas.ChatToolChoiceTypeCustom, schemas.ChatToolChoiceTypeAllowedTools:
			return true
		}
	}
	for _, message := range chat.Input {
		if message.Role == schemas.ChatMessageRoleAssistant && message.ChatAssistantMessage != nil &&
			len(message.ToolCalls) != 0 && message.Reasoning == nil {
			return true
		}
	}
	return false
}
