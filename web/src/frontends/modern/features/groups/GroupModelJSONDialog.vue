<script setup lang="ts">
import { DialogRoot } from 'reka-ui'
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  AppButton,
  AppCheckbox,
  AppDialogContent,
  AppDialogHeader,
  AppNotice,
  AppSelect,
  AppTextArea,
} from '@modern/components/ui'
import type { GroupDraftModel } from './group-create-rules'
import {
  ModelRedirectError,
  parseModelRedirects,
  planModelRedirects,
} from './model-redirect-import'

const props = defineProps<{ models: readonly GroupDraftModel[] }>()
const emit = defineEmits<{ close: []; confirm: [models: GroupDraftModel[]] }>()
const { t, n } = useI18n()
const text = ref('')
const mode = ref<'merge' | 'replace'>('merge')
const replaceRawNames = ref(true)
const overwrite = ref(new Set<string>())
const modes = computed(() =>
  ['merge', 'replace'].map((value) => ({
    value,
    label: t('groupWorkflows.redirects.modes.' + value),
  })),
)
const parsed = computed(() => {
  if (!text.value.trim()) return { entries: [], error: '' }
  try {
    return { entries: parseModelRedirects(text.value), error: '' }
  } catch (error) {
    if (!(error instanceof ModelRedirectError)) throw error
    return { entries: [], error: t('groupWorkflows.redirects.errors.' + error.reason) }
  }
})
const plan = computed(() =>
  planModelRedirects(
    props.models,
    parsed.value.entries,
    mode.value,
    replaceRawNames.value,
    overwrite.value,
  ),
)
const applicable = computed(
  () => Boolean(text.value.trim()) && !parsed.value.error && plan.value.changed,
)
const counts = computed(() => {
  const values = { add: 0, rename: 0, update: 0, unchanged: 0, conflict: 0 }
  for (const row of plan.value.changes) values[row.action]++
  return Object.fromEntries(Object.entries(values).map(([key, value]) => [key, n(value)]))
})
function setOverwrite(name: string, enabled: boolean): void {
  const next = new Set(overwrite.value)
  if (enabled) next.add(name)
  else next.delete(name)
  overwrite.value = next
}
watch([text, mode, () => JSON.stringify(props.models)], () => {
  overwrite.value = new Set()
})
function apply(): void {
  if (applicable.value) emit('confirm', plan.value.next)
}
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
      :title="t('groupWorkflows.redirects.title')"
      :description="t('groupWorkflows.redirects.help')"
    >
      <AppDialogHeader
        :title="t('groupWorkflows.redirects.title')"
        :description="t('groupWorkflows.redirects.help')"
        :close-label="t('ui.close')"
        @close="emit('close')"
      />
      <div class="modern-model-json-body">
        <AppSelect
          v-model="mode"
          :label="t('groupWorkflows.redirects.mode')"
          :options="modes"
          size="sm"
        />
        <AppCheckbox
          v-if="mode === 'merge'"
          v-model="replaceRawNames"
          :label="t('groupWorkflows.redirects.replaceRawNames')"
        />
        <AppNotice v-if="mode === 'replace'" tone="warning">{{
          t('groupWorkflows.redirects.replaceWarning')
        }}</AppNotice>
        <AppTextArea
          v-model="text"
          :label="t('groupWorkflows.redirects.json')"
          :rows="7"
          mono
          spellcheck="false"
          :placeholder="'{\n  &quot;my-model&quot;: &quot;vendor/model-id&quot;\n}'"
          :error="parsed.error || undefined"
        />
        <template v-if="text.trim() && !parsed.error">
          <p class="modern-model-json-summary">
            {{ t('groupWorkflows.redirects.summary', counts) }}
          </p>
          <div class="modern-model-json-preview">
            <div v-for="row in plan.changes" :key="row.name" class="modern-model-json-row">
              <div class="modern-model-json-names">
                <strong>{{ row.name }}</strong>
                <span>{{ t('groupWorkflows.redirects.target') }} {{ row.id }}</span>
                <span
                  v-if="
                    row.previous.length && (row.action === 'update' || row.action === 'conflict')
                  "
                  >{{ t('groupWorkflows.redirects.previous') }} {{ row.previous.join(', ') }}</span
                >
              </div>
              <AppCheckbox
                v-if="row.action === 'conflict' || (mode === 'merge' && row.action === 'update')"
                :model-value="overwrite.has(row.name)"
                :label="t('groupWorkflows.redirects.useIncoming')"
                @update:model-value="setOverwrite(row.name, $event)"
              />
              <span v-else class="modern-model-json-action">{{
                t('groupWorkflows.redirects.actions.' + row.action)
              }}</span>
            </div>
          </div>
          <AppNotice v-if="plan.changes.some((row) => row.action === 'conflict')" tone="warning">{{
            t('groupWorkflows.redirects.conflictHelp')
          }}</AppNotice>
          <AppNotice v-if="plan.removedNames.length" tone="warning">
            {{ t('groupWorkflows.redirects.removedNames', { count: n(plan.removedNames.length) }) }}
            <div class="modern-model-json-removed">{{ plan.removedNames.join(', ') }}</div>
          </AppNotice>
          <AppNotice v-if="!plan.changed">{{ t('groupWorkflows.redirects.noChanges') }}</AppNotice>
        </template>
      </div>
      <footer class="modern-model-json-footer">
        <AppButton @click="emit('close')">{{ t('ui.cancel') }}</AppButton>
        <AppButton variant="primary" :disabled="!applicable" @click="apply">{{
          t('groupWorkflows.confirmSync')
        }}</AppButton>
      </footer>
    </AppDialogContent>
  </DialogRoot>
</template>
<style scoped>
.modern-model-json-body {
  display: grid;
  gap: var(--modern-space-4);
  padding: var(--modern-space-4) var(--modern-space-6);
  overflow-y: auto;
}
.modern-model-json-summary,
.modern-model-json-action,
.modern-model-json-names span {
  color: var(--modern-muted);
  font-size: var(--modern-font-size-small);
}
.modern-model-json-preview {
  max-height: 260px;
  overflow-y: auto;
}
.modern-model-json-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: var(--modern-space-3);
  padding-block: var(--modern-space-3);
  border-bottom: var(--modern-line-width) solid var(--modern-border);
}
.modern-model-json-names {
  display: grid;
  min-width: 0;
  gap: var(--modern-space-1);
  overflow-wrap: anywhere;
}
.modern-model-json-names strong {
  font-size: var(--modern-font-size-secondary);
  font-weight: var(--modern-weight-medium);
}
.modern-model-json-removed {
  max-height: 100px;
  overflow-y: auto;
  overflow-wrap: anywhere;
}
.modern-model-json-footer {
  display: flex;
  justify-content: flex-end;
  flex-wrap: wrap;
  gap: var(--modern-space-3);
  padding: var(--modern-space-4) var(--modern-space-6);
  border-top: var(--modern-line-width) solid var(--modern-border);
}
</style>
