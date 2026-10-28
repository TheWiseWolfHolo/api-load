<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Plus, Trash2 } from '@lucide/vue'
import { transparentMethods, type TransparentPolicy } from '@shared/transparent'
import { transparentMessages } from '@modern/i18n/locales/transparent'
import {
  AppButton,
  AppFormSection,
  AppIconButton,
  AppMultiSelect,
  AppNotice,
  AppSelect,
  AppSwitch,
  AppTextField,
  type SearchSelectOption,
} from '@modern/components/ui'
defineProps<{ groups: readonly SearchSelectOption[]; disabled: boolean; error?: string }>()
const policy = defineModel<TransparentPolicy | undefined>({ required: true })
const { t } = useI18n({ useScope: 'local', messages: transparentMessages })
const enabled = computed({
  get: () => Boolean(policy.value),
  set: (value: boolean) => {
    policy.value = value ? { group_ids: [], methods: [], paths: [] } : undefined
  },
})
const selectedGroups = computed({
  get: () => policy.value?.group_ids.map(String) ?? [],
  set: (values: string[]) => {
    if (policy.value) policy.value.group_ids = values.map(Number)
  },
})
const methods = transparentMethods.map((value) => ({ value, label: value }))
const matches = computed(() => [
  { value: 'exact', label: t('exact') },
  { value: 'subtree', label: t('subtree') },
])
</script>
<template>
  <AppFormSection :title="t('accessTitle')" compact>
    <div class="modern-transparent-setting">
      <span>{{ t('accessEnabled') }}</span
      ><AppSwitch v-model="enabled" :label="t('accessEnabled')" :disabled="disabled" />
    </div>
    <template v-if="policy">
      <p class="modern-transparent-note">{{ t('accessHelp') }}</p>
      <AppMultiSelect
        v-model="selectedGroups"
        :label="t('groups')"
        :options="$props.groups"
        :disabled="disabled"
        :error="error === 'groupsRequired' || error === 'groupsConflict' ? t(error) : undefined"
      />
      <AppMultiSelect
        v-model="policy.methods"
        :label="t('methods')"
        :options="methods"
        :disabled="disabled"
        :error="error === 'methodsRequired' ? t(error) : undefined"
      />
      <div class="modern-transparent-heading">
        <span>{{ t('paths') }}</span
        ><AppButton
          :icon="Plus"
          size="xs"
          variant="text"
          :disabled="disabled"
          @click="policy.paths.push({ path: '', match: 'exact' })"
          >{{ t('addPath') }}</AppButton
        >
      </div>
      <div v-for="(row, index) in policy.paths" :key="index" class="modern-transparent-path">
        <AppTextField
          v-model="row.path"
          :label="t('path')"
          :disabled="disabled"
          :error="error === 'pathsRequired' ? t(error) : undefined"
          placeholder="/v1/embeddings"
        />
        <AppSelect
          v-model="row.match"
          :label="t('match')"
          :options="matches"
          :disabled="disabled"
        />
        <AppIconButton
          :icon="Trash2"
          :label="t('removePath')"
          :disabled="disabled"
          @click="policy.paths.splice(index, 1)"
        />
      </div>
      <AppNotice v-if="error" tone="danger">{{ t(error) }}</AppNotice>
      <p class="modern-transparent-note">{{ t('noModels') }}</p>
    </template>
  </AppFormSection>
</template>
<style scoped>
.modern-transparent-setting,
.modern-transparent-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--modern-space-3);
}
.modern-transparent-path {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr) auto;
  align-items: end;
  gap: var(--modern-space-2);
}
.modern-transparent-note {
  margin: 0;
  color: var(--modern-muted);
  font-size: var(--modern-font-size-small);
}
</style>
