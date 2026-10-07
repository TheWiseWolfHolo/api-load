<script setup lang="ts">
import { DialogRoot } from 'reka-ui'
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { GroupRow } from '@modern/api/groups'
import {
  AppButton,
  AppChannelIcon,
  AppDialogContent,
  AppDialogHeader,
  AppNotice,
  AppSearchSelect,
  AppSelect,
  AppTextField,
} from '@modern/components/ui'
import {
  customIconURL,
  libraryIconNames,
  libraryIconReference,
} from '@modern/components/ui/custom-channel-icons'

const props = defineProps<{ group: GroupRow; value?: string; pending?: boolean; error?: boolean }>()
const emit = defineEmits<{ close: []; save: [value: string] }>()
const { t } = useI18n()
const mode = ref(
  props.value?.startsWith('builtin:')
    ? 'builtin'
    : props.value?.startsWith('lobehub:')
      ? 'lobehub'
      : props.value?.startsWith('data:')
        ? 'image'
        : props.value
          ? 'url'
          : 'auto',
)
const input = ref(props.value?.replace(/^(builtin:|lobehub:)/, '') ?? '')
const image = ref(mode.value === 'image' ? (props.value ?? '') : '')
const imageError = ref(false)
const imagePending = ref(false)
const fileError = ref(false)
const reading = ref(false)
const modes = computed(() =>
  ['auto', 'builtin', 'lobehub', 'url', 'image'].map((value) => ({
    value,
    label: t('groups.icons.' + value),
  })),
)
const options = libraryIconNames.map((name) => ({ value: name, label: name, keywords: [name] }))
const slug = computed(() =>
  input.value
    .trim()
    .toLowerCase()
    .replace(/\.color$/, '-color'),
)
const selection = computed(() =>
  mode.value === 'auto'
    ? ''
    : mode.value === 'builtin'
      ? (libraryIconReference(input.value) ?? 'builtin:' + input.value.trim())
      : mode.value === 'lobehub'
        ? 'lobehub:' + slug.value
        : mode.value === 'image'
          ? image.value
          : input.value.trim(),
)
const valid = computed(
  () =>
    mode.value === 'auto' ||
    (mode.value === 'builtin'
      ? Boolean(libraryIconReference(input.value))
      : Boolean(customIconURL(selection.value))),
)
const previewURL = computed(() => customIconURL(selection.value))
watch(
  previewURL,
  (value) => {
    imagePending.value = Boolean(value)
  },
  { immediate: true },
)
watch(selection, () => {
  imageError.value = false
  fileError.value = false
})
async function choose(event: Event) {
  const field = event.target as HTMLInputElement
  const file = field.files?.[0]
  if (!file) return
  reading.value = true
  fileError.value = false
  try {
    if (!['image/png', 'image/jpeg', 'image/webp'].includes(file.type) || file.size > 10000000)
      throw new Error('INVALID_IMAGE')
    const bitmap = await createImageBitmap(file)
    try {
      const ratio = Math.min(1, 256 / Math.max(bitmap.width, bitmap.height))
      const canvas = document.createElement('canvas')
      canvas.width = Math.max(1, Math.round(bitmap.width * ratio))
      canvas.height = Math.max(1, Math.round(bitmap.height * ratio))
      const context = canvas.getContext('2d')
      if (!context) throw new Error('CANVAS_UNAVAILABLE')
      context.drawImage(bitmap, 0, 0, canvas.width, canvas.height)
      const data = canvas.toDataURL('image/png')
      if (data.length > 350000) throw new Error('IMAGE_TOO_LARGE')
      image.value = data
    } finally {
      bitmap.close()
    }
  } catch {
    fileError.value = true
  } finally {
    reading.value = false
    field.value = ''
  }
}
</script>

<template>
  <DialogRoot
    :open="true"
    @update:open="
      (open) => {
        if (!open && !pending) emit('close')
      }
    "
  >
    <AppDialogContent :title="t('groups.icons.title')" :description="group.name">
      <AppDialogHeader
        :title="t('groups.icons.title')"
        :description="group.name"
        :close-label="t('shell.close')"
        :close-disabled="pending"
        @close="emit('close')"
      />
      <div class="modern-group-icon-editor">
        <AppSelect
          v-model="mode"
          :options="modes"
          :label="t('groups.icons.title')"
          :disabled="pending || reading"
        />
        <AppSearchSelect
          v-if="mode === 'builtin'"
          v-model="input"
          :options="options"
          :label="t('groups.icons.name')"
          :placeholder="t('groups.icons.example')"
          :disabled="pending"
          allow-custom
        />
        <AppTextField
          v-else-if="mode === 'lobehub' || mode === 'url'"
          v-model="input"
          :label="t(mode === 'url' ? 'groups.icons.url' : 'groups.icons.name')"
          :placeholder="mode === 'url' ? 'https://...' : 'deepseek-color'"
          :disabled="pending"
        />
        <label v-else-if="mode === 'image'" class="modern-group-icon-upload"
          >{{ t('groups.icons.choose')
          }}<input
            type="file"
            accept="image/png,image/jpeg,image/webp"
            :disabled="pending || reading"
            @change="choose"
        /></label>
        <p>
          {{
            t(
              mode === 'lobehub'
                ? 'groups.icons.lobeHelp'
                : mode === 'image'
                  ? 'groups.icons.imageHelp'
                  : 'groups.icons.help',
            )
          }}
        </p>
        <div class="modern-group-icon-preview">
          <img
            v-if="previewURL"
            :src="previewURL"
            alt=""
            referrerpolicy="no-referrer"
            @error="
              imageError = true
              imagePending = false
            "
            @load="
              imageError = false
              imagePending = false
            "
          />
          <AppChannelIcon
            v-else
            :icon="group.channelIcon"
            :custom-icon="selection"
            :group-name="group.name"
            :mark="group.channelMark"
            size="hero"
            :tooltip="false"
          />
          <span>{{ group.name }}</span>
        </div>
        <AppNotice v-if="fileError || imageError || error" tone="danger">{{
          t(error ? 'groups.icons.saveFailed' : 'groups.icons.loadFailed')
        }}</AppNotice>
      </div>
      <footer class="modern-group-icon-actions">
        <AppButton :disabled="pending" @click="emit('close')">{{ t('ui.cancel') }}</AppButton
        ><AppButton
          variant="primary"
          :loading="pending || reading"
          :disabled="!valid || imageError || imagePending || reading || pending"
          @click="emit('save', selection)"
          >{{ t('ui.save') }}</AppButton
        >
      </footer>
    </AppDialogContent>
  </DialogRoot>
</template>

<style scoped>
.modern-group-icon-editor {
  display: grid;
  gap: var(--modern-space-4);
  padding: var(--modern-space-6);
  overflow: auto;
}
.modern-group-icon-editor p {
  margin: 0;
  color: var(--modern-muted);
  font-size: var(--modern-font-size-secondary);
}
.modern-group-icon-preview {
  display: flex;
  align-items: center;
  gap: var(--modern-space-4);
  padding: var(--modern-space-4);
  border: var(--modern-line-width) solid var(--modern-border);
  border-radius: var(--modern-radius-panel);
  background: var(--modern-subtle);
}
.modern-group-icon-preview img {
  width: var(--modern-channel-hero);
  height: var(--modern-channel-hero);
  object-fit: contain;
}
.modern-group-icon-upload {
  display: grid;
  gap: var(--modern-space-2);
}
.modern-group-icon-actions {
  display: flex;
  justify-content: flex-end;
  gap: var(--modern-space-2);
  padding: var(--modern-space-4) var(--modern-space-6);
  border-top: var(--modern-line-width) solid var(--modern-border);
}
</style>
