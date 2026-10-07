<script setup lang="ts">
import { RefreshCw, X } from '@lucide/vue'
import { DialogRoot } from 'reka-ui'
import { computed, onScopeDispose, ref } from 'vue'
import { useLoadingActivity } from '@modern/components/ui/loading'
import { useI18n } from 'vue-i18n'
import { discoverGroupModels } from '@modern/api/group-detail'
import {
  AppButton,
  AppCheckbox,
  AppDialogContent,
  AppIconButton,
  AppNotice,
  AppSearchSelect,
  AppSelect,
  AppTextField,
} from '@modern/components/ui'
import { useApiClient } from '@shared/http/client-context'
import type { GroupDraftModel } from './group-create-rules'
import {
  syncModelDraft,
  syncModelConflicts,
  type SyncAddition,
  type SyncMissing,
} from './group-model-sync'

const props = defineProps<{ groupId: number; models: readonly GroupDraftModel[] }>()
const emit = defineEmits<{ close: []; confirm: [models: GroupDraftModel[]] }>()
const { t, n } = useI18n()
const client = useApiClient()
const current = props.models.map((model) => ({ ...model }))
const additions = ref<SyncAddition[]>([])
const missing = ref<SyncMissing[]>([])
const upstream = ref<string[]>([])
const loading = ref(false)
const loaded = ref(false)
const failed = ref(false)
const search = ref('')
useLoadingActivity(loading)
let controller: AbortController | undefined
const options = computed(() => upstream.value.map((id) => ({ value: id, label: id })))
const actions = computed(() =>
  ['keep', 'replace', 'remove'].map((value) => ({
    value,
    label: t('groupWorkflows.missingActions.' + value),
  })),
)
const next = computed(() => syncModelDraft(current, additions.value, missing.value))
const conflicts = computed(() => syncModelConflicts(current, next.value))
const added = computed(() => additions.value.filter((row) => row.selected).length)
const removed = computed(() => missing.value.filter((row) => row.action === 'remove').length)
const replaced = computed(() => missing.value.filter((row) => row.action === 'replace').length)
const invalidReplacement = computed(() =>
  missing.value.some(
    (row) => row.action === 'replace' && !upstream.value.includes(row.replacement),
  ),
)
const canApply = computed(
  () =>
    loaded.value &&
    !loading.value &&
    !failed.value &&
    !conflicts.value.length &&
    !invalidReplacement.value &&
    Boolean(added.value || removed.value || replaced.value),
)
function matches(...values: string[]): boolean {
  const needle = search.value.trim().toLocaleLowerCase()
  return !needle || values.some((value) => value.toLocaleLowerCase().includes(needle))
}
const visibleAdditions = computed(() => additions.value.filter((row) => matches(row.id, row.alias)))
const visibleMissing = computed(() =>
  missing.value.filter((row) => matches(row.model.id, row.model.alias)),
)
const retained = computed(() => current.length - missing.value.length)
const allSelected = computed({
  get: () =>
    Boolean(visibleAdditions.value.length) && visibleAdditions.value.every((row) => row.selected),
  set: (selected: boolean) => {
    for (const row of visibleAdditions.value) row.selected = selected
  },
})
async function load(): Promise<void> {
  controller?.abort()
  const request = new AbortController()
  controller = request
  loading.value = true
  failed.value = false
  loaded.value = false
  try {
    const candidates = await discoverGroupModels(client, props.groupId, request.signal)
    if (request.signal.aborted) return
    // 目录建议不证明上游提供该模型，只采用本次实时返回的条目。
    upstream.value = [
      ...new Set(
        candidates.filter((row) => row.sources.includes('live')).map((row) => row.id.trim()),
      ),
    ]
    const ids = new Set(upstream.value)
    const configured = new Set(current.map((row) => row.id.trim()))
    let key = Math.max(-1, ...current.map((row) => row.key)) + 1
    additions.value = upstream.value
      .filter((id) => !configured.has(id))
      .map((id) => ({
        id,
        alias: '',
        key: key++,
        origin: 'discovery',
        selected: false,
      }))
    missing.value = current
      .filter((row) => !ids.has(row.id.trim()))
      .map((model) => ({
        model,
        action: 'keep',
        replacement: '',
      }))
    loaded.value = true
  } catch {
    if (!request.signal.aborted) failed.value = true
  } finally {
    if (!request.signal.aborted) loading.value = false
  }
}
function apply(): void {
  if (canApply.value) emit('confirm', next.value)
}
void load()
onScopeDispose(() => controller?.abort())
</script>

<template>
  <DialogRoot
    open
    @update:open="
      (open) => {
        if (!open) emit('close')
      }
    "
  >
    <AppDialogContent
      size="wide"
      :title="t('groupWorkflows.syncModels')"
      :description="t('groupWorkflows.syncDescription')"
    >
      <header class="modern-model-sync-heading">
        <h2>{{ t('groupWorkflows.syncModels') }}</h2>
        <AppIconButton :icon="X" :label="t('ui.close')" @click="emit('close')" />
      </header>
      <div class="modern-model-sync-body">
        <p>{{ t('groupWorkflows.syncDescription') }}</p>
        <AppNotice v-if="loading">{{ t('collection.loading') }}</AppNotice>
        <AppNotice v-else-if="failed" tone="danger">
          {{ t('groupCreate.discoveryFailed') }}
          <template #actions
            ><AppButton size="sm" @click="load">{{ t('ui.retry') }}</AppButton></template
          >
        </AppNotice>
        <template v-else-if="loaded">
          <AppNotice>{{ t('groupWorkflows.syncRetained', { count: n(retained) }) }}</AppNotice>
          <AppTextField v-model="search" :label="t('groupWorkflows.syncSearch')" size="sm" />
          <section v-if="additions.length" class="modern-model-sync-section">
            <div class="modern-model-sync-section-heading">
              <h3>{{ t('groupWorkflows.additions', { count: n(additions.length) }) }}</h3>
              <AppCheckbox
                v-model="allSelected"
                :label="t('groupWorkflows.syncSelectVisible')"
                :disabled="!visibleAdditions.length"
              />
            </div>
            <p>{{ t('groupWorkflows.syncAddHelp') }}</p>
            <ul class="modern-model-sync-list">
              <li v-for="row in visibleAdditions" :key="row.key" class="modern-model-sync-row">
                <AppCheckbox v-model="row.selected" :label="row.id" />
                <AppTextField
                  v-model="row.alias"
                  :label="t('groupWorkflows.syncPublicName')"
                  :placeholder="row.id"
                  :disabled="!row.selected"
                  size="sm"
                />
              </li>
            </ul>
          </section>
          <section v-if="missing.length" class="modern-model-sync-section">
            <h3>{{ t('groupWorkflows.syncMissing', { count: n(missing.length) }) }}</h3>
            <p>{{ t('groupWorkflows.syncMissingHelp') }}</p>
            <ul class="modern-model-sync-list">
              <li v-for="row in visibleMissing" :key="row.model.key" class="modern-model-sync-row">
                <div class="modern-model-sync-identity">
                  <strong>{{ row.model.alias || row.model.id }}</strong>
                  <span>{{ row.model.id }}</span>
                </div>
                <div class="modern-model-sync-fields">
                  <AppSelect
                    v-model="row.action"
                    :label="t('groupWorkflows.syncAction')"
                    :options="actions"
                    size="sm"
                  />
                  <AppSearchSelect
                    v-if="row.action === 'replace'"
                    v-model="row.replacement"
                    :options="options"
                    :label="t('groupWorkflows.syncReplacement')"
                    size="sm"
                  />
                </div>
              </li>
            </ul>
          </section>
          <AppNotice v-if="!additions.length && !missing.length">{{
            t('groupWorkflows.noModelChanges')
          }}</AppNotice>
          <AppNotice v-if="conflicts.length" tone="danger"
            >{{ t('groupWorkflows.syncNameConflicts') }} {{ conflicts.join('、') }}</AppNotice
          >
          <AppNotice v-if="invalidReplacement" tone="warning">{{
            t('groupWorkflows.syncChooseReplacement')
          }}</AppNotice>
          <AppNotice v-if="!upstream.length" tone="warning">{{
            t('groupWorkflows.syncEmptyUpstream')
          }}</AppNotice>
          <AppNotice v-if="removed" tone="warning">{{
            t('groupWorkflows.syncRemovalWarning')
          }}</AppNotice>
          <AppNotice v-if="canApply && !next.length" tone="warning">{{
            t('groupWorkflows.clearModels')
          }}</AppNotice>
        </template>
      </div>
      <footer class="modern-model-sync-actions">
        <span>{{
          t('groupWorkflows.syncSummary', {
            added: n(added),
            removed: n(removed),
            replaced: n(replaced),
          })
        }}</span>
        <AppButton @click="emit('close')">{{ t('ui.cancel') }}</AppButton>
        <AppButton variant="primary" :icon="RefreshCw" :disabled="!canApply" @click="apply">{{
          t('groupWorkflows.confirmSync')
        }}</AppButton>
      </footer>
    </AppDialogContent>
  </DialogRoot>
</template>

<style scoped>
.modern-model-sync-heading,
.modern-model-sync-section-heading,
.modern-model-sync-actions {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--modern-space-3);
  flex-wrap: wrap;
}
.modern-model-sync-heading {
  padding: var(--modern-space-4) var(--modern-space-6);
  border-bottom: var(--modern-line-width) solid var(--modern-border);
}
.modern-model-sync-heading h2 {
  font-size: var(--modern-font-size-section);
  font-weight: var(--modern-weight-semibold);
}
.modern-model-sync-body,
.modern-model-sync-section,
.modern-model-sync-fields {
  display: grid;
  gap: var(--modern-space-3);
  min-width: 0;
}
.modern-model-sync-body {
  padding: var(--modern-space-4) var(--modern-space-6);
  overflow-y: auto;
}
.modern-model-sync-body p,
.modern-model-sync-identity span,
.modern-model-sync-actions > span {
  color: var(--modern-muted);
  font-size: var(--modern-font-size-small);
}
.modern-model-sync-section {
  padding-block: var(--modern-space-3);
  border-top: var(--modern-line-width) solid var(--modern-border);
}
.modern-model-sync-section h3 {
  font-size: var(--modern-font-size-secondary);
  font-weight: var(--modern-weight-medium);
}
.modern-model-sync-list {
  list-style: none;
  padding: 0;
  margin: 0;
  max-height: 340px;
  overflow-y: auto;
}
.modern-model-sync-row {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  align-items: center;
  gap: var(--modern-space-4);
  padding-block: var(--modern-space-3);
  border-bottom: var(--modern-line-width) solid var(--modern-border);
  overflow-wrap: anywhere;
}
.modern-model-sync-identity {
  display: grid;
  gap: var(--modern-space-1);
  min-width: 0;
}
.modern-model-sync-actions {
  padding: var(--modern-space-4) var(--modern-space-6);
  border-top: var(--modern-line-width) solid var(--modern-border);
}
.modern-model-sync-actions > span {
  flex: 1;
}
@media (max-width: 760px) {
  .modern-model-sync-row {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
