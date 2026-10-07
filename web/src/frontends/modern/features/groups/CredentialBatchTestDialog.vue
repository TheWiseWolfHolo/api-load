<script setup lang="ts">
import { useQuery } from '@tanstack/vue-query'
import { DialogRoot } from 'reka-ui'
import { computed, onScopeDispose, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  getGroupCredentials,
  getGroupModels,
  getGroupSettings,
  groupModelsKey,
  groupSettingsKey,
  type CredentialRow,
} from '@modern/api/group-detail'
import { testCredential, type CredentialTestResult } from '@modern/api/credential-actions'
import {
  AppButton,
  AppDialogContent,
  AppDialogHeader,
  AppNotice,
  AppSearchSelect,
  AppSelect,
} from '@modern/components/ui'
import AppDraftGuard from '@modern/components/AppDraftGuard.vue'
import { useApiClient } from '@shared/http/client-context'
import { protocolLabel } from '@modern/i18n/protocols'
import { defaultTestModel, groupTestModelOptions, resolveTestModel } from './group-test-models'

const props = defineProps<{ groupId: number; rows?: CredentialRow[] }>()
const emit = defineEmits<{ close: []; changed: []; pending: [value: boolean] }>()
const { t, n } = useI18n()
const client = useApiClient()
const settings = useQuery({
  queryKey: groupSettingsKey(props.groupId),
  queryFn: ({ signal }) => getGroupSettings(client, props.groupId, signal),
})
const models = useQuery({
  queryKey: groupModelsKey(props.groupId),
  queryFn: ({ signal }) => getGroupModels(client, props.groupId, signal),
})
const targets = useQuery({
  queryKey: ['modern', 'credential-test-targets', props.groupId],
  enabled: props.rows === undefined,
  staleTime: 0,
  queryFn: async ({ signal }) => {
    const rows: CredentialRow[] = []
    let pages = 1
    for (let page = 1; page <= pages; page++) {
      const data = await getGroupCredentials(
        client,
        props.groupId,
        { q: '', status: '', proxy: '', reset: '', sort: 'oldest', pageSize: 100, page },
        signal,
      )
      if (page === 1) pages = Math.ceil(data.total / data.pageSize)
      rows.push(...data.items)
    }
    return [...new Map(rows.map((row) => [row.id, row])).values()]
  },
})
const rows = computed(() => props.rows ?? targets.data.value ?? [])
const model = ref('')
const protocol = ref('')
let initialized = false
watch(
  [settings.data, models.data],
  ([value, items]) => {
    if (!value || !items || initialized) return
    protocol.value = value.validationProtocol ?? value.validationProtocols[0] ?? ''
    model.value = defaultTestModel(items)
    initialized = true
  },
  { immediate: true },
)
const protocols = computed(() =>
  (settings.data.value?.validationProtocols ?? []).map((value) => ({
    value,
    label: protocolLabel(value, t),
  })),
)
const modelOptions = computed(() => groupTestModelOptions(models.data.value ?? []))
const effectiveModel = computed(
  () => model.value.trim() || defaultTestModel(models.data.value ?? []),
)
const upstreamModel = computed(() =>
  resolveTestModel(models.data.value ?? [], effectiveModel.value),
)
type Entry = {
  row: CredentialRow
  state: 'queued' | 'running' | 'done' | 'error' | 'cancelled'
  result?: CredentialTestResult
}
const entries = ref<Entry[]>([])
const pending = ref(false)
const stopped = ref(false)
const completed = computed(
  () => entries.value.filter((item) => item.state === 'done' || item.state === 'error').length,
)
const passed = computed(
  () => entries.value.filter((item) => item.result?.outcome === 'passed').length,
)
const failed = computed(
  () => entries.value.filter((item) => item.result?.outcome === 'failed').length,
)
const uncertain = computed(
  () =>
    entries.value.filter(
      (item) => item.result?.outcome === 'inconclusive' || item.state === 'error',
    ).length,
)
const loading = computed(
  () =>
    settings.isPending.value ||
    models.isPending.value ||
    (props.rows === undefined && targets.isFetching.value),
)
const loadFailed = computed(
  () =>
    settings.isError.value ||
    models.isError.value ||
    (props.rows === undefined && targets.isError.value),
)
let controller: AbortController | undefined
watch(pending, (value) => emit('pending', value), { flush: 'sync' })
async function run(): Promise<void> {
  if (
    pending.value ||
    loading.value ||
    loadFailed.value ||
    !rows.value.length ||
    !effectiveModel.value ||
    !protocol.value
  )
    return
  const activeModel = upstreamModel.value
  const activeProtocol = protocol.value
  const abort = new AbortController()
  controller = abort
  entries.value = rows.value.map((row) => ({ row, state: 'queued' }))
  stopped.value = false
  pending.value = true
  let cursor = 0
  async function worker(): Promise<void> {
    while (!abort.signal.aborted && cursor < entries.value.length) {
      const entry = entries.value[cursor++]!
      entry.state = 'running'
      try {
        entry.result = await testCredential(
          client,
          props.groupId,
          entry.row.id,
          activeProtocol,
          activeModel,
          abort.signal,
        )
        entry.state = 'done'
      } catch {
        entry.state = abort.signal.aborted ? 'cancelled' : 'error'
      }
    }
  }
  try {
    await Promise.all([worker(), worker()])
  } finally {
    for (const entry of entries.value) if (entry.state === 'queued') entry.state = 'cancelled'
    pending.value = false
    emit('changed')
  }
}
function stop(): void {
  stopped.value = true
  controller?.abort()
}
function close(): void {
  if (!pending.value) emit('close')
}
async function reload(): Promise<void> {
  await Promise.all([
    settings.refetch(),
    models.refetch(),
    ...(props.rows === undefined ? [targets.refetch()] : []),
  ])
}
onScopeDispose(() => {
  controller?.abort()
  emit('pending', false)
})
</script>

<template>
  <DialogRoot
    :open="true"
    @update:open="
      (value) => {
        if (!value) close()
      }
    "
  >
    <AppDialogContent
      :title="t('groupWorkflows.keyBatch.title')"
      :description="t('groupWorkflows.keyBatch.help')"
    >
      <AppDialogHeader
        :title="t('groupWorkflows.keyBatch.title')"
        :close-label="t('shell.close')"
        :close-disabled="pending"
        @close="close"
      />
      <div class="key-batch">
        <AppNotice>{{
          t(
            props.rows
              ? 'groupWorkflows.keyBatch.selectedScope'
              : 'groupWorkflows.keyBatch.allScope',
            { count: n(rows.length) },
          )
        }}</AppNotice>
        <AppSelect
          v-model="protocol"
          :label="t('groupDetail.validationProtocol')"
          :options="protocols"
          :disabled="pending || loading || protocols.length === 1"
        />
        <AppSearchSelect
          v-model="model"
          :label="t('groupDetail.validationModel')"
          :options="modelOptions"
          :description="t('credentialCards.testModelHelp')"
          allow-custom
          :disabled="pending || loading"
        />
        <AppNotice v-if="!model.trim() && effectiveModel">{{
          t('credentialCards.testDefaultModel', { model: effectiveModel })
        }}</AppNotice>
        <AppNotice v-if="effectiveModel && upstreamModel !== effectiveModel">{{
          t('credentialCards.testUpstreamModel', { model: upstreamModel })
        }}</AppNotice>
        <p class="key-batch-help">{{ t('groupWorkflows.keyBatch.help') }}</p>
        <AppNotice v-if="loading">{{ t('ui.loading') }}</AppNotice>
        <AppNotice v-if="!loading && !protocols.length" tone="warning">{{
          t('credentialCards.noTestProtocol')
        }}</AppNotice>
        <AppNotice v-if="loadFailed" tone="danger"
          >{{ t('groups.edit.loadFailed')
          }}<template #actions
            ><AppButton :disabled="loading || pending" @click="reload">{{
              t('ui.retry')
            }}</AppButton></template
          ></AppNotice
        >
        <div v-if="entries.length" role="status" aria-live="polite">
          {{
            t('groupWorkflows.keyBatch.progress', {
              completed: n(completed),
              total: n(entries.length),
              passed: n(passed),
              failed: n(failed),
              uncertain: n(uncertain),
            })
          }}
          <span v-if="stopped"> · {{ t('groupWorkflows.keyBatch.stopped') }}</span>
        </div>
        <div v-if="entries.length" class="key-batch-results">
          <div v-for="entry in entries" :key="entry.row.id" class="key-batch-row">
            <span>{{ entry.row.label }}</span>
            <span
              >{{
                entry.result
                  ? t('credentialCards.testResult.' + entry.result.outcome)
                  : t('groupWorkflows.keyBatch.states.' + entry.state)
              }}<template v-if="entry.result">
                · {{ entry.result.latency }} ms<template v-if="entry.result.reason">
                  · {{ t('credentialCards.testReason.' + entry.result.reason) }}</template
                ></template
              ></span
            >
          </div>
        </div>
        <div class="key-batch-actions">
          <AppButton :disabled="pending" @click="close">{{ t('shell.close') }}</AppButton>
          <AppButton v-if="pending" variant="outline" :disabled="stopped" @click="stop">{{
            t('groupWorkflows.keyBatch.stop')
          }}</AppButton>
          <AppButton
            v-else
            variant="primary"
            :disabled="loading || loadFailed || !rows.length || !effectiveModel || !protocol"
            @click="run"
            >{{
              t(entries.length ? 'groupWorkflows.keyBatch.rerun' : 'groupWorkflows.keyBatch.start')
            }}</AppButton
          >
        </div>
      </div>
    </AppDialogContent>
  </DialogRoot>
  <AppDraftGuard :dirty="false" :pending="pending" />
</template>

<style scoped>
.key-batch {
  display: grid;
  min-height: 0;
  overflow: auto;
  gap: var(--modern-space-4);
  padding: var(--modern-space-5) var(--modern-space-6);
}
.key-batch-help {
  margin: 0;
  color: var(--modern-muted);
  font-size: var(--modern-font-size-small);
}
.key-batch-results {
  max-height: 280px;
  overflow: auto;
}
.key-batch-row {
  display: flex;
  flex-wrap: wrap;
  justify-content: space-between;
  gap: var(--modern-space-2);
  padding: var(--modern-space-3) 0;
  border-bottom: 1px solid var(--modern-border);
  overflow-wrap: anywhere;
}
.key-batch-actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: var(--modern-space-2);
}
</style>
