<script setup lang="ts">
import { Check, Copy } from '@lucide/vue'
import { computed, onScopeDispose, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { AppButton, AppCopyValue } from '@modern/components/ui'
import { modelIDsText } from './model-redirect-import'
const props = defineProps<{ ids: readonly string[]; label: string; disabled?: boolean }>()
const { t } = useI18n()
const text = computed(() => modelIDsText(props.ids))
const copied = ref(false)
let timer: ReturnType<typeof setTimeout> | undefined
function reset(): void {
  clearTimeout(timer)
  copied.value = false
}
function done(): void {
  reset()
  copied.value = true
  timer = setTimeout(reset, 2000)
}
watch(text, reset)
onScopeDispose(reset)
</script>
<template>
  <AppCopyValue :value="text" :label="label" @copied="done">
    <template #trigger="{ copy, pending }">
      <AppButton
        :icon="copied ? Check : Copy"
        size="sm"
        variant="ghost"
        :disabled="disabled || !text"
        :loading="pending"
        @click="copy()"
      >
        {{ copied ? t('ui.copy.success') : label }}
      </AppButton>
    </template>
  </AppCopyValue>
</template>
