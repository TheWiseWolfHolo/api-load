import type { ChannelType } from "@/types/models";

export type ProviderIconType = "chatgpt" | "codex" | "gemini" | "claude";

export interface ProviderMeta {
  displayName: string;
  icon: string;
  iconType: ProviderIconType;
}

const providerDisplayNames: Record<string, string> = {
  openai: "OpenAI",
  "openai-response": "OpenAI Responses",
  gemini: "Gemini",
  anthropic: "Anthropic",
};

const providerIconTypes: Record<string, ProviderIconType> = {
  openai: "chatgpt",
  "openai-response": "codex",
  gemini: "gemini",
  anthropic: "claude",
};

export function getProviderIconType(channelType?: ChannelType | string): ProviderIconType {
  if (!channelType) {
    return "chatgpt";
  }
  return providerIconTypes[channelType] ?? "chatgpt";
}

export function getProviderMeta(channelType?: ChannelType | string): ProviderMeta {
  const iconType = getProviderIconType(channelType);
  if (!channelType) {
    return { displayName: "OpenAI", icon: `/provider-icons/${iconType}.svg`, iconType };
  }
  return {
    displayName: providerDisplayNames[channelType] ?? channelType,
    icon: `/provider-icons/${iconType}.svg`,
    iconType,
  };
}
