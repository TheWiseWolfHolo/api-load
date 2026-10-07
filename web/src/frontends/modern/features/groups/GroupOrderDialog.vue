<script setup lang="ts">
import { ArrowDown, ArrowUp, GripVertical } from '@lucide/vue'
import { DialogRoot } from 'reka-ui'
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import type { GroupRow } from '@modern/api/groups'
import {
  AppButton,
  AppDialogContent,
  AppDialogHeader,
  AppIcon,
  AppIconButton,
  AppNotice,
} from '@modern/components/ui'

const props = defineProps<{
  groups: GroupRow[]
  order: readonly number[]
  error?: boolean
  pending?: boolean
}>()
const emit = defineEmits<{ close: []; save: [ids: number[]] }>()
const { t } = useI18n()
const positions = new Map(props.order.map((id, index) => [id, index]))
const rows = ref(
  [...props.groups].sort(
    (a, b) => (positions.get(a.id) ?? Infinity) - (positions.get(b.id) ?? Infinity) || a.id - b.id,
  ),
)
const dragging = ref<number>()
const hover = ref<number>()
function move(id: number, index: number) {
  const old = rows.value.findIndex((row) => row.id === id)
  if (old < 0 || !Number.isFinite(index)) return
  const next = [...rows.value]
  const [row] = next.splice(old, 1)
  if (!row) return
  next.splice(Math.max(0, Math.min(next.length, Math.trunc(index))), 0, row)
  rows.value = next
}
function start(event: DragEvent, id: number) {
  dragging.value = id
  if (event.dataTransfer) {
    event.dataTransfer.effectAllowed = 'move'
    event.dataTransfer.setData('text/plain', String(id))
  }
}
function drop(index: number) {
  if (dragging.value !== undefined) move(dragging.value, index)
  dragging.value = undefined
  hover.value = undefined
}
</script>

<template>
  <DialogRoot
    :open="true"
    @update:open="
      (open) => {
        if (!open) emit('close')
      }
    "
  >
    <AppDialogContent :title="t('groups.order.title')" :description="t('groups.order.help')">
      <AppDialogHeader
        :title="t('groups.order.title')"
        :close-label="t('shell.close')"
        :close-disabled="pending"
        @close="emit('close')"
      />
      <div class="modern-group-order-body">
        <p>{{ t('groups.order.help') }}</p>
        <AppNotice v-if="error" tone="danger">{{ t('groups.order.failed') }}</AppNotice>
        <ol class="modern-group-order-list">
          <li
            v-for="(row, index) in rows"
            :key="row.id"
            :class="{ 'is-over': hover === row.id && dragging !== row.id }"
            @dragover.prevent="hover = row.id"
            @drop.prevent="drop(index)"
          >
            <button
              type="button"
              class="modern-group-order-grip"
              draggable="true"
              :aria-label="t('groups.order.drag', { name: row.name })"
              @dragstart="start($event, row.id)"
              @dragend="
                dragging = undefined
                hover = undefined
              "
            >
              <AppIcon :icon="GripVertical" />
            </button>
            <span class="modern-group-order-name">{{ row.name }}</span>
            <input
              type="number"
              :value="index + 1"
              min="1"
              :max="rows.length"
              :aria-label="t('groups.order.position', { name: row.name })"
              @change="move(row.id, Number(($event.target as HTMLInputElement).value) - 1)"
            />
            <AppIconButton
              :icon="ArrowUp"
              :label="t('groups.order.up')"
              :disabled="index === 0"
              size="sm"
              @click="move(row.id, index - 1)"
            />
            <AppIconButton
              :icon="ArrowDown"
              :label="t('groups.order.down')"
              :disabled="index === rows.length - 1"
              size="sm"
              @click="move(row.id, index + 1)"
            />
          </li>
        </ol>
      </div>
      <footer class="modern-group-order-actions">
        <AppButton :disabled="pending" @click="emit('close')">{{ t('ui.cancel') }}</AppButton
        ><AppButton
          variant="primary"
          :loading="pending"
          :disabled="pending"
          @click="
            emit(
              'save',
              rows.map((row) => row.id),
            )
          "
          >{{ t('ui.save') }}</AppButton
        >
      </footer>
    </AppDialogContent>
  </DialogRoot>
</template>

<style scoped>
.modern-group-order-body {
  padding: var(--modern-space-4) var(--modern-space-6);
  overflow: auto;
}
.modern-group-order-body p {
  margin: 0 0 var(--modern-space-4);
  color: var(--modern-muted);
  font-size: var(--modern-font-size-secondary);
}
.modern-group-order-list {
  list-style: none;
  padding: 0;
  margin: 0;
}
.modern-group-order-list li {
  display: flex;
  align-items: center;
  gap: var(--modern-space-2);
  min-height: var(--modern-control-nav);
  padding-block: var(--modern-space-1);
  border-bottom: var(--modern-line-width) solid var(--modern-border);
}
.modern-group-order-list li.is-over {
  background: var(--modern-control-hover);
}
.modern-group-order-grip {
  display: grid;
  place-items: center;
  border: 0;
  padding: var(--modern-space-2);
  color: var(--modern-muted);
  background: transparent;
  cursor: grab;
}
.modern-group-order-grip:focus-visible {
  outline: var(--modern-focus-width) solid var(--modern-accent);
  outline-offset: var(--modern-focus-offset);
}
.modern-group-order-name {
  flex: 1;
  min-width: 0;
  overflow-wrap: anywhere;
}
.modern-group-order-list input {
  width: var(--modern-inline-number-width);
  min-height: var(--modern-control-sm);
  border: var(--modern-line-width) solid var(--modern-control-border);
  border-radius: var(--modern-radius-small);
  padding-inline: var(--modern-space-2);
  color: var(--modern-text);
  background: var(--modern-surface);
  font: inherit;
}
.modern-group-order-actions {
  display: flex;
  justify-content: flex-end;
  gap: var(--modern-space-2);
  padding: var(--modern-space-4) var(--modern-space-6);
  border-top: var(--modern-line-width) solid var(--modern-border);
}
</style>
