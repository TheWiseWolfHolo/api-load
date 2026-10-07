import type { GroupModel } from '@modern/api/group-detail'
import type { SearchSelectOption } from '@modern/components/ui'

// Public names remain distinct even when several aliases share an upstream ID.
export function groupTestModelOptions(models: readonly GroupModel[]): SearchSelectOption[] {
  return models.map((model) => ({
    value: model.clientModel,
    label: model.clientModel,
    description: model.clientModel === model.id ? undefined : model.id,
    keywords: [model.id],
  }))
}

export function defaultTestModel(models: readonly GroupModel[]): string {
  return models[0]?.clientModel ?? ''
}

export function resolveTestModel(models: readonly GroupModel[], name: string): string {
  const value = name.trim()
  return models.find((model) => model.clientModel === value)?.id ?? value
}
