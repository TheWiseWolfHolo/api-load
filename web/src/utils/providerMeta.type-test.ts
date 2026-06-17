import {
  getProviderIconType,
  getProviderMeta,
  type ProviderIconType,
} from "./providerMeta";

const chatgptIcon: ProviderIconType = getProviderIconType("openai");
const codexIcon: ProviderIconType = getProviderIconType("openai-response");
const geminiIcon: ProviderIconType = getProviderIconType("gemini");
const claudeIcon: ProviderIconType = getProviderIconType("anthropic");
const fallbackIcon: ProviderIconType = getProviderIconType(undefined);
const displayName: string = getProviderMeta("openai-response").displayName;

void chatgptIcon;
void codexIcon;
void geminiIcon;
void claudeIcon;
void fallbackIcon;
void displayName;
