<script setup lang="ts">
import { computed } from 'vue'
import { useQuery } from '@tanstack/vue-query'
import { useI18n } from 'vue-i18n'
import { useApiClient } from '@shared/http/client-context'
import { getGroupSettings, groupSettingsKey } from '@modern/api/group-detail'
import { transparentMessages } from '@modern/i18n/locales/transparent'
import { AppBadge, AppCopyValue } from '@modern/components/ui'
const props = defineProps<{ groupId: number }>()
const { t } = useI18n({ useScope: 'local', messages: transparentMessages })
const client = useApiClient()
const query = useQuery(
  computed(() => ({
    queryKey: groupSettingsKey(props.groupId),
    queryFn: ({ signal }: { signal: AbortSignal }) =>
      getGroupSettings(client, props.groupId, signal),
  })),
)
const endpoint = computed(() => `${globalThis.location.origin}/proxy/${props.groupId}`)
</script>
<template>
  <div
    v-if="query.data.value?.effective.transparent_proxy?.enabled"
    class="modern-transparent-connection"
  >
    <AppBadge size="xs">{{ t('title') }}</AppBadge
    ><span>{{ t('endpoint') }}</span
    ><AppCopyValue :value="endpoint" :label="t('copy')" wrap />
  </div>
</template>
<style scoped>
.modern-transparent-connection {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: var(--modern-space-2);
  font-size: var(--modern-font-size-small);
  overflow-wrap: anywhere;
}
</style>
