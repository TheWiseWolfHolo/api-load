import type { GroupDraftModel } from './group-create-rules'

export interface SyncAddition extends GroupDraftModel {
  selected: boolean
}
export interface SyncMissing {
  model: GroupDraftModel
  action: 'keep' | 'remove' | 'replace'
  replacement: string
}

export function syncModelDraft(
  current: readonly GroupDraftModel[],
  additions: readonly SyncAddition[],
  missing: readonly SyncMissing[],
): GroupDraftModel[] {
  const changes = new Map(missing.map((row) => [row.model.key, row]))
  const result: GroupDraftModel[] = []
  for (const model of current) {
    const change = changes.get(model.key)
    if (change?.action === 'remove') continue
    if (change?.action === 'replace') {
      result.push({
        ...model,
        id: change.replacement.trim(),
        alias: model.alias.trim() || model.id.trim(),
      })
    } else result.push({ ...model })
  }
  for (const { selected, ...model } of additions) {
    if (selected) result.push({ ...model, id: model.id.trim(), alias: model.alias.trim() })
  }
  return result
}

// 已有多来源映射继续保留，只阻止本次新增的撞名和重复映射。
export function syncModelConflicts(
  current: readonly GroupDraftModel[],
  next: readonly GroupDraftModel[],
): string[] {
  function names(models: readonly GroupDraftModel[]): Map<string, Set<string>> {
    const result = new Map<string, Set<string>>()
    for (const model of models) {
      const name = model.alias.trim() || model.id.trim()
      const ids = result.get(name) ?? new Set<string>()
      ids.add(model.id.trim())
      result.set(name, ids)
    }
    return result
  }
  const previous = names(current)
  const conflicts = new Set<string>()
  const seen = new Set<string>()
  for (const model of next) {
    const name = model.alias.trim() || model.id.trim()
    const pair = JSON.stringify([name, model.id.trim()])
    if (seen.has(pair)) conflicts.add(name)
    seen.add(pair)
  }
  for (const [name, ids] of names(next)) {
    if (ids.size > 1 && [...ids].some((id) => !previous.get(name)?.has(id))) conflicts.add(name)
  }
  return [...conflicts]
}
