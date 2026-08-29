<script setup lang="ts">
import type { ResourceValidationGroup, UpstreamResource } from "@/types/models";
import { PulseOutline, RefreshOutline } from "@vicons/ionicons5";
import {
  NButton,
  NDescriptions,
  NDescriptionsItem,
  NDrawer,
  NDrawerContent,
  NIcon,
  NPopconfirm,
  NSelect,
  NTag,
  NTimeline,
  NTimelineItem,
} from "naive-ui";
import { computed, ref, watch } from "vue";
import { useI18n } from "vue-i18n";

const props = defineProps<{
  show: boolean;
  resource: UpstreamResource | null;
  validationGroups: ResourceValidationGroup[];
  validating: boolean;
  mutating: boolean;
}>();
const emit = defineEmits<{
  "update:show": [value: boolean];
  validate: [resource: UpstreamResource, groupId: number];
  forceRecover: [resource: UpstreamResource];
  toggleScheduling: [resource: UpstreamResource];
}>();
const { t } = useI18n();
const validationGroupID = ref<number | null>(null);
type TimelineType = "default" | "success" | "error" | "info";
interface DiagnosticEvent {
  key: string;
  label: string;
  value?: string;
  type: TimelineType;
}

const validationOptions = computed(() =>
  props.validationGroups.map(group => ({
    label: `${group.display_name || group.name} · ${group.channel_type}`,
    value: group.id,
  }))
);
const events = computed<DiagnosticEvent[]>(() => {
  const resource = props.resource;
  if (!resource) {
    return [];
  }
  const timeline: DiagnosticEvent[] = [
    {
      key: "failure",
      label: t("resourcePools.lastFailure"),
      value: resource.last_failure_at,
      type: "error",
    },
    {
      key: "success",
      label: t("resourcePools.lastSuccess"),
      value: resource.last_success_at,
      type: "success",
    },
    { key: "used", label: t("resourcePools.lastUsed"), value: resource.last_used_at, type: "info" },
    {
      key: "updated",
      label: t("resourcePools.lastUpdated"),
      value: resource.updated_at,
      type: "default",
    },
    {
      key: "created",
      label: t("resourcePools.createdAt"),
      value: resource.created_at,
      type: "default",
    },
  ];
  return timeline
    .filter((event): event is DiagnosticEvent & { value: string } => Boolean(event.value))
    .sort((a, b) => new Date(b.value || 0).getTime() - new Date(a.value || 0).getTime());
});

watch(
  () => [props.show, props.validationGroups] as const,
  () => {
    if (!props.validationGroups.some(group => group.id === validationGroupID.value)) {
      validationGroupID.value = props.validationGroups[0]?.id ?? null;
    }
  },
  { immediate: true }
);

function formatDate(value?: string): string {
  if (!value) {
    return "—";
  }
  return new Intl.DateTimeFormat(undefined, { dateStyle: "medium", timeStyle: "short" }).format(
    new Date(value)
  );
}

function runValidation() {
  if (props.resource && validationGroupID.value) {
    emit("validate", props.resource, validationGroupID.value);
  }
}
</script>

<template>
  <n-drawer
    :show="show"
    width="min(560px, 100vw)"
    placement="right"
    :native-scrollbar="true"
    @update:show="value => emit('update:show', value)"
  >
    <n-drawer-content
      v-if="resource"
      closable
      :title="resource.name || `#${resource.id}`"
      :body-content-style="{ padding: '20px' }"
    >
      <div class="diagnostic-heading">
        <div>
          <code>{{ resource.masked_key }}</code>
          <span>{{ t("resourcePools.diagnosticSubtitle") }}</span>
        </div>
        <div class="diagnostic-tags">
          <n-tag :type="resource.status === 'active' ? 'success' : 'error'" size="small">
            {{ t(`resourcePools.status_${resource.status}`) }}
          </n-tag>
          <n-tag :type="resource.enabled ? 'info' : 'warning'" size="small">
            {{
              resource.enabled
                ? t("resourcePools.schedulingAllowed")
                : t("resourcePools.manualDisabled")
            }}
          </n-tag>
        </div>
      </div>

      <section class="diagnostic-section">
        <h3>{{ t("resourcePools.currentState") }}</h3>
        <n-descriptions :column="2" label-placement="top" size="small">
          <n-descriptions-item :label="t('resourcePools.priority')">
            {{ resource.priority }}
          </n-descriptions-item>
          <n-descriptions-item :label="t('resourcePools.weight')">
            {{ resource.weight }}
          </n-descriptions-item>
          <n-descriptions-item :label="t('resourcePools.successfulCalls')">
            {{ resource.request_count }}
          </n-descriptions-item>
          <n-descriptions-item :label="t('resourcePools.totalFailures')">
            {{ resource.total_failure_count }}
          </n-descriptions-item>
          <n-descriptions-item :label="t('resourcePools.consecutiveFailures')">
            {{ resource.failure_count }}
          </n-descriptions-item>
          <n-descriptions-item :label="t('resourcePools.cooldown')">
            {{ formatDate(resource.global_cooldown_until) }}
          </n-descriptions-item>
        </n-descriptions>
        <div v-if="resource.disabled_reason" class="failure-reason">
          <strong>{{ t("resourcePools.currentFailureReason") }}</strong>
          <p>{{ resource.disabled_reason }}</p>
        </div>
      </section>

      <section class="diagnostic-section">
        <h3>{{ t("resourcePools.recoveryActions") }}</h3>
        <p class="section-help">{{ t("resourcePools.recoveryActionsHelp") }}</p>
        <n-select
          v-if="validationGroups.length > 1"
          v-model:value="validationGroupID"
          :options="validationOptions"
          :placeholder="t('resourcePools.validationRoute')"
        />
        <div class="drawer-actions">
          <n-button
            type="primary"
            :disabled="!validationGroupID || mutating"
            :loading="validating"
            @click="runValidation"
          >
            <template #icon><n-icon :component="PulseOutline" /></template>
            {{ t("resourcePools.validateAndRecover") }}
          </n-button>
          <n-button :disabled="validating || mutating" @click="emit('toggleScheduling', resource)">
            {{ resource.enabled ? t("resourcePools.pauseScheduling") : t("resourcePools.enable") }}
          </n-button>
          <n-popconfirm @positive-click="emit('forceRecover', resource)">
            <template #trigger>
              <n-button type="warning" :disabled="validating || mutating">
                <template #icon><n-icon :component="RefreshOutline" /></template>
                {{ t("resourcePools.forceRecover") }}
              </n-button>
            </template>
            {{ t("resourcePools.forceRecoverConfirm") }}
          </n-popconfirm>
        </div>
      </section>

      <section class="diagnostic-section">
        <h3>{{ t("resourcePools.activityTimeline") }}</h3>
        <n-timeline>
          <n-timeline-item
            v-for="event in events"
            :key="event.key"
            :type="event.type"
            :title="event.label"
            :time="formatDate(event.value)"
          />
        </n-timeline>
      </section>
    </n-drawer-content>
  </n-drawer>
</template>

<style scoped>
.diagnostic-heading,
.diagnostic-tags,
.drawer-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}
.diagnostic-heading {
  justify-content: space-between;
  align-items: flex-start;
  padding-bottom: 18px;
  border-bottom: 1px solid var(--border-color-light);
}
.diagnostic-heading > div:first-child {
  display: grid;
  gap: 4px;
}
.diagnostic-heading code {
  color: var(--text-primary);
  font-family: var(--font-mono);
}
.diagnostic-heading span,
.section-help {
  color: var(--text-secondary);
}
.diagnostic-tags,
.drawer-actions {
  flex-wrap: wrap;
}
.diagnostic-section {
  padding: 20px 0;
  border-bottom: 1px solid var(--border-color-light);
}
.diagnostic-section:last-child {
  border-bottom: 0;
}
.diagnostic-section h3 {
  margin: 0 0 12px;
  color: var(--text-primary);
  font-size: 1rem;
}
.section-help {
  margin: -4px 0 14px;
  font-size: 0.82rem;
  line-height: 1.55;
}
.failure-reason {
  margin-top: 14px;
  padding: 12px 14px;
  border: 1px solid var(--error-border);
  border-radius: var(--border-radius-md);
  background: var(--error-bg);
}
.failure-reason strong {
  color: var(--error-color);
  font-size: 0.78rem;
}
.failure-reason p {
  margin: 5px 0 0;
  color: var(--text-primary);
  font-family: var(--font-mono);
  font-size: 0.76rem;
  line-height: 1.5;
  overflow-wrap: anywhere;
}
.drawer-actions {
  margin-top: 12px;
}
@media (max-width: 640px) {
  .diagnostic-heading {
    gap: 12px;
    flex-direction: column;
  }
  .drawer-actions > :deep(*) {
    flex: 1 1 100%;
  }
}
</style>
