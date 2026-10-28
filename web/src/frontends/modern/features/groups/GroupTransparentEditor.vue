<script setup lang="ts">
import { computed, watch } from 'vue'
import { useQuery } from '@tanstack/vue-query'
import { credentialDetailKey, getCredentialDetail } from '@modern/api/credential-actions'
import { useI18n } from 'vue-i18n'
import { useApiClient } from '@shared/http/client-context'
import { transparentProxyError, type TransparentProxy } from '@shared/transparent'
import { getGroupCredentials } from '@modern/api/group-detail'
import { transparentMessages } from '@modern/i18n/locales/transparent'
import {
  AppCopyValue,
  AppFormSection,
  AppNotice,
  AppSearchSelect,
  AppSelect,
  AppSwitch,
  AppTextField,
} from '@modern/components/ui'
const props = defineProps<{
  groupId: number
  eligible: boolean
  disabled: boolean
  attempted: boolean
}>()
const model = defineModel<TransparentProxy>({ required: true })
const { t } = useI18n({ useScope: 'local', messages: transparentMessages })
const client = useApiClient()
const endpoint = computed(() => `${globalThis.location.origin}/proxy/${props.groupId}`)
const issue = computed(() => transparentProxyError(model.value))
const fixedID = computed({
  get: () => (model.value.fixed_credential_id ? String(model.value.fixed_credential_id) : ''),
  set: (id: string) => {
    model.value.fixed_credential_id = Number(id)
  },
})
watch(
  () => model.value.strategy,
  (strategy) => {
    if (strategy === 'rotate') model.value.fixed_credential_id = 0
  },
)
const selected = useQuery(
  computed(() => ({
    queryKey: credentialDetailKey(props.groupId, model.value.fixed_credential_id),
    queryFn: ({ signal }: { signal: AbortSignal }) =>
      getCredentialDetail(client, props.groupId, model.value.fixed_credential_id, signal),
    enabled:
      model.value.enabled &&
      model.value.strategy === 'fixed' &&
      model.value.fixed_credential_id > 0,
  })),
)
const selectedOption = computed(() =>
  selected.data.value
    ? {
        value: String(selected.data.value.id),
        label: selected.data.value.name
          ? `${selected.data.value.name} · ${selected.data.value.mask}`
          : selected.data.value.mask,
      }
    : undefined,
)
const strategies = computed(() => [
  { value: 'rotate', label: t('rotate') },
  { value: 'fixed', label: t('fixed') },
])
async function credentials(query: string, signal: AbortSignal) {
  const result = await getGroupCredentials(
    client,
    props.groupId,
    { q: query, status: '', sort: 'priority', page: 1, pageSize: 100, proxy: '', reset: '' },
    signal,
  )
  return result.items.map((row) => ({
    value: String(row.id),
    label: row.name ? `${row.name} · ${row.mask}` : row.mask,
    description: `#${row.id}`,
    disabled: !row.enabled,
  }))
}
</script>
<template>
  <AppFormSection :title="t('title')" compact>
    <div class="modern-transparent-setting">
      <span>{{ t('enabled') }}</span
      ><AppSwitch v-model="model.enabled" :label="t('enabled')" :disabled="disabled || !eligible" />
    </div>
    <AppNotice v-if="!eligible" tone="info">{{ t('notSupported') }}</AppNotice>
    <template v-if="model.enabled">
      <AppTextField
        v-model="model.base_url"
        :label="t('base')"
        :description="t('baseHelp')"
        :disabled="disabled"
        :error="attempted && issue === 'invalidBase' ? t(issue) : undefined"
      />
      <div class="modern-transparent-fields">
        <AppTextField
          v-model="model.auth_header"
          :label="t('authHeader')"
          :disabled="disabled"
          :error="attempted && issue === 'invalidAuth' ? t(issue) : undefined"
        />
        <AppTextField v-model="model.auth_prefix" :label="t('authPrefix')" :disabled="disabled" />
      </div>
      <p class="modern-transparent-note">{{ t('authHelp') }}</p>
      <AppSelect
        v-model="model.strategy"
        :label="t('strategy')"
        :options="strategies"
        :disabled="disabled"
        :error="attempted && issue === 'strategyRequired' ? t(issue) : undefined"
      />
      <AppSearchSelect
        v-if="model.strategy === 'fixed'"
        v-model="fixedID"
        :label="t('credential')"
        :placeholder="t('credentialSearch')"
        :load-options="credentials"
        :selected-option="selectedOption"
        :description="t('searchLimit')"
        :disabled="disabled"
        :error="attempted && issue === 'credentialRequired' ? t(issue) : undefined"
      />
      <p class="modern-transparent-note">{{ t('fixedHelp') }}</p>
      <div class="modern-transparent-endpoint">
        <span>{{ t('endpoint') }}</span
        ><AppCopyValue :value="endpoint" :label="t('copy')" wrap />
      </div>
      <p class="modern-transparent-note">{{ t('inactive') }}</p>
      <AppNotice tone="info">{{ t('behavior') }} {{ t('supported') }}</AppNotice>
    </template>
  </AppFormSection>
</template>
<style scoped>
.modern-transparent-setting {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--modern-space-3);
}
.modern-transparent-fields {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: var(--modern-space-3);
}
.modern-transparent-note {
  margin: 0;
  color: var(--modern-muted);
  font-size: var(--modern-font-size-small);
}
.modern-transparent-endpoint {
  display: grid;
  gap: var(--modern-space-2);
  overflow-wrap: anywhere;
  font-size: var(--modern-font-size-small);
}
</style>
