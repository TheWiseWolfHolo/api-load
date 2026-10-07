import type { ApiClient } from '@shared/http/client'
import { integer, list, record, text } from './response'

export interface GroupPresentation {
  order: number[]
  icons: Record<string, string>
}
export const groupPresentationKey = ['modern', 'group-presentation'] as const
function read(value: unknown): GroupPresentation {
  const data = record(value)
  const icons = Object.fromEntries(
    Object.entries(record(data.icons)).map(([id, icon]) => [
      String(integer(Number(id), 1)),
      text(icon),
    ]),
  )
  return { order: list(data.order).map((id) => integer(id, 1)), icons }
}
export async function getGroupPresentation(client: ApiClient, signal: AbortSignal) {
  return read(await client.request('/api/groups/presentation', { signal }))
}
export async function updateGroupPresentation(
  client: ApiClient,
  patch: { order?: number[]; group_id?: number; icon?: string },
  signal: AbortSignal,
) {
  return read(
    await client.request('/api/groups/presentation', { method: 'PUT', json: patch, signal }),
  )
}
