import lobeNames from './lobe-icon-names.json'
import { channelIconNames } from './channel-icons'
const availableLobeIcons = new Set(lobeNames)
export const libraryIconNames = [
  ...new Set([...channelIconNames, ...lobeNames.map((name) => name.replace(/-color$/, ''))]),
].sort()
export function libraryIconReference(name: string): string | undefined {
  const value = name
    .trim()
    .toLowerCase()
    .replace(/\.color$/, '-color')
  const normalized = aliases[value] ?? value
  if (channelIconNames.includes(normalized)) return 'builtin:' + normalized
  const slug = [normalized + '-color', normalized].find((candidate) =>
    availableLobeIcons.has(candidate),
  )
  return slug ? 'lobehub:' + slug : undefined
}
const aliases: Record<string, string> = {
  nvidia: 'nvidia',
  ollama: 'ollama',
  minimax: 'minimax',
  qwen: 'qwen',
  vercel: 'vercel',
  sarvam: 'sarvam',
  mistral: 'mistral',
  gemini: 'gemini',
  deepseek: 'deepseek',
  fireworks: 'fireworks',
  cohere: 'cohere',
  groq: 'groq',
  cerebras: 'cerebras',
  openrouter: 'openrouter',
  kimi: 'moonshot',
  moonshot: 'moonshot',
  glm: 'zhipu',
  zhipu: 'zhipu',
  siliconflow: 'siliconcloud',
  siliconcloud: 'siliconcloud',
  ali: 'alibabacloud',
  bailian: 'alibabacloud',
  alibaba: 'alibabacloud',
  volcengine: 'volcengine',
  huggingface: 'huggingface',
  nebius: 'nebius',
  cline: 'cline',
  anthropic: 'anthropic',
  claude: 'claude',
  openai: 'openai',
  codex: 'codex',
  grok: 'xai',
  xai: 'xai',
  opencode: 'opencode',
  antigravity: 'antigravity',
  azure: 'azure',
  bedrock: 'bedrock',
  vertex: 'vertexai',
  vertexai: 'vertexai',
  parasail: 'parasail',
  wafer: 'wafer',
}
export function matchGroupIcon(name: string): string | undefined {
  const normalized = name
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
  const match = Object.keys(aliases)
    .sort((a, b) => b.length - a.length)
    .find((key) => normalized === key || normalized.startsWith(key))
  return match ? aliases[match] : undefined
}
export function customIconURL(value: string): string | undefined {
  if (/^lobehub:[a-z0-9][a-z0-9-]{0,63}$/.test(value))
    return 'https://unpkg.com/@lobehub/icons-static-svg@1.95.1/icons/' + value.slice(8) + '.svg'
  if (/^data:image\/png;base64,[a-zA-Z0-9+/=]+$/.test(value) && value.length <= 350000) return value
  try {
    const url = new URL(value)
    if (url.protocol === 'https:' && !url.username && !url.password && value.length <= 2048)
      return url.href
  } catch {
    return undefined
  }
  return undefined
}

export function automaticGroupIcon(name: string): string | undefined {
  const matched = matchGroupIcon(name)
  return matched ? libraryIconReference(matched) : undefined
}
