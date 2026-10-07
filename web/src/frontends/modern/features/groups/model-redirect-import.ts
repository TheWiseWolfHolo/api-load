import type { GroupDraftModel } from './group-create-rules'

export interface ModelRedirect {
  name: string
  id: string
}
export class ModelRedirectError extends Error {
  readonly reason: 'invalidJSON' | 'invalidObject' | 'invalidName' | 'duplicateName' | 'tooLarge'
  constructor(reason: ModelRedirectError['reason']) {
    super(reason)
    this.reason = reason
  }
}

export function parseModelRedirects(source: string): ModelRedirect[] {
  const text = source
    .trim()
    .replace(/^```(?:json)?\s*\n([\s\S]*?)\n```$/u, '$1')
    .trim()
  if (new TextEncoder().encode(text).length > 1_048_576) throw new ModelRedirectError('tooLarge')
  let value: unknown
  try {
    value = JSON.parse(text)
  } catch {
    throw new ModelRedirectError('invalidJSON')
  }
  if (
    !value ||
    typeof value !== 'object' ||
    Array.isArray(value) ||
    Object.values(value).some((item) => typeof item !== 'string')
  )
    throw new ModelRedirectError('invalidObject')
  // JSON.parse 会覆盖重复键，逐个读取字符串对以检查重复的公开名称。
  const pairs = /\s*"((?:\\.|[^"\\])*)"\s*:\s*"((?:\\.|[^"\\])*)"\s*/gy
  let offset = 1
  const seen = new Set<string>()
  const entries: ModelRedirect[] = []
  while (offset < text.length - 1) {
    pairs.lastIndex = offset
    const match = pairs.exec(text)
    if (!match) {
      if (/^\s*\}$/u.test(text.slice(offset))) break
      throw new ModelRedirectError('invalidObject')
    }
    const name = (JSON.parse('"' + match[1] + '"') as string).trim()
    const id = (JSON.parse('"' + match[2] + '"') as string).trim()
    if (!name || !id || /[\u0000-\u001f\u007f]/u.test(name + id))
      throw new ModelRedirectError('invalidName')
    if (seen.has(name)) throw new ModelRedirectError('duplicateName')
    seen.add(name)
    entries.push({ name, id })
    if (entries.length > 20_000) throw new ModelRedirectError('tooLarge')
    offset = pairs.lastIndex
    if (text[offset] !== ',') break
    offset++
  }
  return entries
}

export function modelIDsText(ids: readonly string[]): string {
  return [...new Set(ids.map((id) => id.trim()).filter(Boolean))].join(',')
}
const publicName = (model: GroupDraftModel) => model.alias.trim() || model.id.trim()

export function planModelRedirects(
  current: readonly GroupDraftModel[],
  entries: readonly ModelRedirect[],
  mode: 'merge' | 'replace',
  replaceRawNames: boolean,
  overwrite: ReadonlySet<string>,
) {
  let next = mode === 'replace' ? [] : current.map((model) => ({ ...model }))
  let key = Math.max(-1, ...current.map((model) => model.key)) + 1
  const importedNames = new Set(entries.map((entry) => entry.name))
  const changes: {
    name: string
    id: string
    previous: string[]
    action: 'add' | 'rename' | 'update' | 'unchanged' | 'conflict'
  }[] = []
  for (const entry of entries) {
    const matches = current.filter((model) => publicName(model) === entry.name)
    const exact = matches.length > 0 && matches.every((model) => model.id.trim() === entry.id)
    const row: GroupDraftModel = {
      id: entry.id,
      alias: entry.name === entry.id ? '' : entry.name,
      key: key++,
      origin: 'manual',
    }
    if (mode === 'replace') {
      next.push(row)
      changes.push({
        ...entry,
        previous: matches.map((model) => model.id),
        action: exact ? 'unchanged' : matches.length ? 'update' : 'add',
      })
    } else if (exact) {
      changes.push({ ...entry, previous: [], action: 'unchanged' })
    } else if (matches.length) {
      if (overwrite.has(entry.name)) {
        const position = next.findIndex((model) => publicName(model) === entry.name)
        next = next.filter((model) => publicName(model) !== entry.name)
        next.splice(position < 0 ? next.length : position, 0, { ...row, key: matches[0]!.key })
      }
      changes.push({
        ...entry,
        previous: matches.map((model) => model.id),
        action: overwrite.has(entry.name) ? 'update' : 'conflict',
      })
    } else {
      const rawIndex =
        replaceRawNames && !importedNames.has(entry.id)
          ? next.findIndex(
              (model) => model.id.trim() === entry.id && publicName(model) === entry.id,
            )
          : -1
      if (rawIndex >= 0) next.splice(rawIndex, 1, { ...row, key: next[rawIndex]!.key })
      else next.push(row)
      changes.push({
        ...entry,
        previous: rawIndex >= 0 ? [entry.id] : [],
        action: rawIndex >= 0 ? 'rename' : 'add',
      })
    }
  }
  const names = new Set(next.map(publicName))
  const removedNames = [
    ...new Set(current.filter((model) => !names.has(publicName(model))).map(publicName)),
  ]
  const signature = (models: readonly GroupDraftModel[]) =>
    JSON.stringify(models.map(({ id, alias }) => [id.trim(), alias.trim()]))
  return { next, changes, removedNames, changed: signature(current) !== signature(next) }
}
