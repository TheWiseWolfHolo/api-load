import type { ApiClient } from '@shared/http/client'
import { boolean, integer, list, record, text } from './response'

export const serviceCapabilitiesKey = (id: number) =>
  ['modern', 'service-capabilities', id] as const
const modalities = (value: unknown) => (value == null ? [] : list(value).map(text))

export async function getServiceCapabilities(client: ApiClient, id: number, signal: AbortSignal) {
  const data = record(await client.request(`/api/groups/${id}/services`, { signal }))
  return {
    truncated: boolean(data.verification_truncated),
    windowDays: integer(data.verification_window_days),
    scanned: integer(data.verification_scanned_records),
    routes: list(data.routes).map((item) => {
      const row = record(item)
      return {
        protocol: text(row.protocol),
        operation: text(row.operation),
        mode: text(row.mode),
        method: text(row.method),
        path: text(row.path),
        output: text(row.output),
        input: modalities(row.input),
        query: modalities(row.query_input),
        documents: modalities(row.document_input),
        limitations: list(row.limitations).map(text),
      }
    }),
    models: list(data.models).map((item) => {
      const row = record(item)
      return {
        model: text(row.model),
        input: modalities(row.official_input),
        output: modalities(row.official_output),
        provider: modalities(row.provider_input),
        query: modalities(row.query_input),
        documents: modalities(row.document_input),
        source: text(row.source),
        date: text(row.checked_at),
        verification: list(row.account_verification).map((item) => {
          const evidence = record(item)
          return {
            credentialId: integer(evidence.credential_id),
            current: boolean(evidence.current_identity),
            requestId: text(evidence.request_id),
            at: integer(evidence.tested_at_ms),
            status: text(evidence.status),
            format: text(evidence.format),
            inputType: evidence.input_type === undefined ? '' : text(evidence.input_type),
            input: modalities(evidence.input),
            query: modalities(evidence.query),
            documents: modalities(evidence.documents),
            dimensions:
              evidence.requested_dimensions === undefined
                ? 0
                : integer(evidence.requested_dimensions, Number.MIN_SAFE_INTEGER),
            result: text(evidence.result_check),
          }
        }),
      }
    }),
  }
}
