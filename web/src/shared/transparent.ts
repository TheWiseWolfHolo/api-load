import { InvalidResponseError } from './http/errors'

export const transparentMethods = [
  'GET',
  'POST',
  'PUT',
  'PATCH',
  'DELETE',
  'HEAD',
  'OPTIONS',
] as const
export interface TransparentProxy {
  enabled: boolean
  base_url: string
  auth_header: string
  auth_prefix: string
  strategy: '' | 'rotate' | 'fixed'
  fixed_credential_id: number
}
export interface TransparentPolicy {
  group_ids: number[]
  methods: string[]
  paths: { path: string; match: 'exact' | 'subtree' }[]
}
function object(value: unknown): Record<string, unknown> {
  if (!value || typeof value !== 'object' || Array.isArray(value)) throw new InvalidResponseError()
  return value as Record<string, unknown>
}
function string(value: unknown): string {
  if (typeof value !== 'string') throw new InvalidResponseError()
  return value
}
function ids(value: unknown): number[] {
  if (!Array.isArray(value) || value.some((id) => !Number.isSafeInteger(id) || id <= 0))
    throw new InvalidResponseError()
  return [...value]
}
export function readTransparentProxy(value: unknown): TransparentProxy {
  const row = object(value)
  if (
    typeof row.enabled !== 'boolean' ||
    !Number.isSafeInteger(row.fixed_credential_id) ||
    Number(row.fixed_credential_id) < 0 ||
    !['', 'rotate', 'fixed'].includes(string(row.strategy))
  )
    throw new InvalidResponseError()
  return {
    enabled: row.enabled,
    base_url: string(row.base_url),
    auth_header: string(row.auth_header),
    auth_prefix: string(row.auth_prefix),
    strategy: row.strategy as TransparentProxy['strategy'],
    fixed_credential_id: Number(row.fixed_credential_id),
  }
}
export function readTransparentPolicy(value: unknown): TransparentPolicy {
  const row = object(value)
  if (!Array.isArray(row.methods) || !Array.isArray(row.paths)) throw new InvalidResponseError()
  return {
    group_ids: ids(row.group_ids),
    methods: row.methods.map(string),
    paths: row.paths.map((value) => {
      const item = object(value)
      if (item.match !== 'exact' && item.match !== 'subtree') throw new InvalidResponseError()
      return { path: string(item.path), match: item.match }
    }),
  }
}
export function cloneTransparentPolicy(policy: TransparentPolicy): TransparentPolicy {
  return {
    group_ids: [...policy.group_ids],
    methods: [...policy.methods],
    paths: policy.paths.map((path) => ({ ...path })),
  }
}
export function canonicalTransparentPolicy(policy?: TransparentPolicy) {
  if (!policy) return undefined
  return {
    group_ids: [...policy.group_ids].sort((a, b) => a - b),
    methods: [...policy.methods].sort(),
    paths: policy.paths
      .map((row) => ({ ...row }))
      .sort((a, b) => (a.path + a.match).localeCompare(b.path + b.match)),
  }
}
export function transparentPolicyError(
  policy: TransparentPolicy | undefined,
  models: readonly string[],
  protocols: readonly string[],
  groups: readonly number[],
  groupsRestricted: boolean,
  costRules: number,
): string {
  if (!policy) return ''
  if (models.length) return 'modelConflict'
  if (!protocols.includes('transparent-http')) return 'protocolConflict'
  if (costRules) return 'costConflict'
  if (
    !policy.group_ids.length ||
    policy.group_ids.some((id) => !Number.isSafeInteger(id) || id <= 0)
  )
    return 'groupsRequired'
  if ((groupsRestricted || groups.length) && policy.group_ids.some((id) => !groups.includes(id)))
    return 'groupsConflict'
  if (
    !policy.methods.length ||
    policy.methods.some(
      (method) => !transparentMethods.includes(method as (typeof transparentMethods)[number]),
    )
  )
    return 'methodsRequired'
  if (
    !policy.paths.length ||
    policy.paths.some(
      ({ path }) =>
        !path.startsWith('/') ||
        /[%?#\\\p{Cc}]/u.test(path) ||
        path.includes('//') ||
        (path.length > 1 && path.endsWith('/')) ||
        path.split('/').some((part) => part === '.' || part === '..'),
    )
  )
    return 'pathsRequired'
  return ''
}
export function transparentProxyError(value: TransparentProxy): string {
  if (!value.enabled) return ''
  if (value.base_url !== value.base_url.trim()) return 'invalidBase'
  const rawPath = value.base_url.match(/^https?:\/\/[^/]*(\/[^?#]*)?/u)?.[1] ?? ''
  if (
    /[%\\\p{Cc}]/u.test(rawPath) ||
    rawPath.includes('//') ||
    rawPath.split('/').some((part) => part === '.' || part === '..')
  )
    return 'invalidBase'
  try {
    const url = new URL(value.base_url)
    if (
      !['http:', 'https:'].includes(url.protocol) ||
      !url.hostname ||
      url.username ||
      url.password ||
      url.search ||
      url.hash
    )
      return 'invalidBase'
  } catch {
    return 'invalidBase'
  }
  if (
    !/^[!#$%&'*+.^_`|~\w-]+$/u.test(value.auth_header) ||
    /\p{Cc}/u.test(value.auth_prefix) ||
    [
      'host',
      'content-length',
      'transfer-encoding',
      'connection',
      'proxy-authorization',
      'proxy-connection',
      'upgrade',
      'te',
      'trailer',
    ].includes(value.auth_header.toLowerCase())
  )
    return 'invalidAuth'
  if (value.strategy !== 'rotate' && value.strategy !== 'fixed') return 'strategyRequired'
  if (
    value.strategy === 'fixed' &&
    (!Number.isSafeInteger(value.fixed_credential_id) || value.fixed_credential_id <= 0)
  )
    return 'credentialRequired'
  return ''
}
