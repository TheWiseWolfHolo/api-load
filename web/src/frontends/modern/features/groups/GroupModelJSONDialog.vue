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
      class="modern-model-json-dialog"
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
        <div
          class="modern-model-json-workspace"
          :class="{ 'has-preview': text.trim() && !parsed.error }"
        >
          <AppTextArea
            v-model="text"
            class="modern-model-json-input"
            :label="t('groupWorkflows.redirects.json')"
            :rows="5"
            mono
            spellcheck="false"
            :placeholder="'{\n  &quot;my-model&quot;: &quot;vendor/model-id&quot;\n}'"
            :error="parsed.error || undefined"
          />
          <section v-if="text.trim() && !parsed.error" class="modern-model-json-review">
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
                    >{{ t('groupWorkflows.redirects.previous') }}
                    {{ row.previous.join(', ') }}</span
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
            <AppNotice
              v-if="plan.changes.some((row) => row.action === 'conflict')"
              tone="warning"
              >{{ t('groupWorkflows.redirects.conflictHelp') }}</AppNotice
            >
            <AppNotice v-if="plan.removedNames.length" tone="warning">
              {{
                t('groupWorkflows.redirects.removedNames', { count: n(plan.removedNames.length) })
              }}
              <div class="modern-model-json-removed">{{ plan.removedNames.join(', ') }}</div>
            </AppNotice>
            <AppNotice v-if="!plan.changed">{{
              t('groupWorkflows.redirects.noChanges')
            }}</AppNotice>
          </section>
        </div>
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
:global(.modern-model-json-dialog) {
  --modern-dialog-top: min(4dvh, 32px);
  height: calc(100dvh - 2 * var(--modern-dialog-top));
  max-height: calc(100dvh - 2 * var(--modern-dialog-top));
}
.modern-model-json-body {
  display: flex;
  flex-direction: column;
  flex: 1 1 auto;
  min-height: 0;
  gap: var(--modern-space-4);
  padding: var(--modern-space-4) var(--modern-space-6);
  overflow-y: auto;
}
.modern-model-json-body > * {
  flex-shrink: 0;
}
.modern-model-json-body > .modern-model-json-workspace {
  flex: 1 1 auto;
  min-height: min(300px, 42dvh);
}
.modern-model-json-workspace.has-preview {
  display: grid;
  grid-template-columns: minmax(0, 0.9fr) minmax(0, 1.25fr);
  gap: var(--modern-space-5);
}
.modern-model-json-input {
  display: flex;
  flex-direction: column;
  min-height: 0;
  height: 100%;
}
.modern-model-json-input :deep(textarea) {
  flex: 1;
  height: 100%;
  min-height: 160px;
}
.modern-model-json-review {
  display: flex;
  flex-direction: column;
  min-height: 0;
  gap: var(--modern-space-3);
}
.modern-model-json-review > :not(.modern-model-json-preview) {
  flex-shrink: 0;
}
.modern-model-json-summary {
  margin: 0;
}
.modern-model-json-summary,
.modern-model-json-action,
.modern-model-json-names span {
  color: var(--modern-muted);
  font-size: var(--modern-font-size-small);
}
.modern-model-json-preview {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
}
@media (max-width: 820px) {
  .modern-model-json-workspace.has-preview {
    display: flex;
    flex-direction: column;
  }
  .modern-model-json-input {
    flex: none;
    height: auto;
  }
  .modern-model-json-review {
    flex: none;
    min-height: 280px;
  }
  .modern-model-json-preview {
    flex: none;
    max-height: 45dvh;
    min-height: 240px;
  }
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
  flex-shrink: 0;
  justify-content: flex-end;
  flex-wrap: wrap;
  gap: var(--modern-space-3);
  padding: var(--modern-space-4) var(--modern-space-6);
  border-top: var(--modern-line-width) solid var(--modern-border);
}
</style>
