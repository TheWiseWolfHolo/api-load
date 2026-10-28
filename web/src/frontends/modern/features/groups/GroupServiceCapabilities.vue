<script setup lang="ts">
import { computed, ref } from 'vue'
import { useQuery } from '@tanstack/vue-query'
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'
import { Search } from '@lucide/vue'
import { useApiClient } from '@shared/http/client-context'
import { getServiceCapabilities, serviceCapabilitiesKey } from '@modern/api/service-capabilities'
import { serviceMessages } from '@modern/i18n/locales/service-capabilities'
import {
  AppBadge,
  AppButton,
  AppCollectionState,
  AppTextField,
  AppProtocolTag,
} from '@modern/components/ui'

const props = defineProps<{ groupId: number }>()
const { t, te, locale } = useI18n({ useScope: 'local', messages: serviceMessages })
const client = useApiClient()
const search = ref('')
const query = useQuery(
  computed(() => ({
    queryKey: serviceCapabilitiesKey(props.groupId),
    queryFn: ({ signal }: { signal: AbortSignal }) =>
      getServiceCapabilities(client, props.groupId, signal),
  })),
)
const models = computed(
  () =>
    query.data.value?.models.filter((model) =>
      model.model.toLowerCase().includes(search.value.trim().toLowerCase()),
    ) ?? [],
)
const modality = (values: string[]) =>
  values
    .map((value) => (te('modalities.' + value) ? t('modalities.' + value) : value))
    .join(' / ') || t('unknown')
const date = (value: number) =>
  new Intl.DateTimeFormat(locale.value, { dateStyle: 'short', timeStyle: 'short' }).format(value)
defineExpose({ refresh: () => query.refetch() })
</script>
<template>
  <section class="modern-services" :aria-label="t('title')">
    <AppCollectionState v-if="query.isPending.value" :title="t('loading')" loading />
    <AppCollectionState v-else-if="query.isError.value" :title="t('failed')" error
      ><AppButton @click="query.refetch()">{{ t('retry') }}</AppButton></AppCollectionState
    >
    <template v-else-if="query.data.value">
      <div class="modern-services-heading">
        <h2>{{ t('routes') }}</h2>
        <AppBadge>{{ query.data.value.routes.length }}</AppBadge>
      </div>
      <div class="modern-services-routes">
        <article
          v-for="route in query.data.value.routes"
          :key="route.protocol + route.operation + route.method + route.path"
          class="modern-services-route"
        >
          <div class="modern-services-heading">
            <AppProtocolTag :protocol="route.protocol" /><AppBadge
              size="xs"
              :variant="route.mode === 'native' ? 'soft' : 'outline'"
              >{{
                t(
                  route.protocol === 'transparent-http'
                    ? 'transparent'
                    : route.mode === 'native'
                      ? 'native'
                      : 'converted',
                )
              }}</AppBadge
            >
          </div>
          <p
            class="modern-services-path"
            :class="{ 'modern-services-path-transparent': route.protocol === 'transparent-http' }"
          >
            <strong>{{ route.method }}</strong> <code>{{ route.path }}</code>
          </p>
          <p>
            <template v-if="route.protocol === 'transparent-http'"
              >{{ t('rawRequest') }} → </template
            ><template v-else-if="!route.query.length">{{ modality(route.input) }} → </template
            ><template v-else>{{ t('output') }} · </template>{{ modality([route.output]) }}
          </p>
          <dl v-if="route.query.length || route.documents.length" class="modern-services-facts">
            <div>
              <dt>{{ t('query') }}</dt>
              <dd>{{ modality(route.query) }}</dd>
            </div>
            <div>
              <dt>{{ t('documents') }}</dt>
              <dd>{{ modality(route.documents) }}</dd>
            </div>
          </dl>
          <ul v-if="route.limitations.length" class="modern-services-limitations">
            <li v-for="limit in route.limitations" :key="limit">
              {{ te('limits.' + limit) ? t('limits.' + limit) : limit }}
            </li>
          </ul>
        </article>
      </div>
      <div class="modern-services-heading">
        <h2>{{ t('models') }}</h2>
        <AppBadge>{{ query.data.value.models.length }}</AppBadge>
      </div>
      <AppTextField
        v-model="search"
        :label="t('search')"
        :placeholder="t('search')"
        :icon="Search"
        type="search"
        label-hidden
      />
      <p v-if="query.data.value.truncated" class="modern-services-note">
        {{ t('truncated', { count: query.data.value.scanned }) }}
      </p>
      <AppCollectionState
        v-if="!models.length"
        :title="
          t(
            search
              ? 'noMatches'
              : query.data.value.routes.some((route) => route.protocol === 'transparent-http')
                ? 'transparentNoModels'
                : 'noModels',
          )
        "
      />
      <details v-for="model in models" :key="model.model" class="modern-services-model">
        <summary>
          <code>{{ model.model }}</code
          ><AppBadge size="xs">{{
            t(model.verification.length ? 'recorded' : 'noRecorded')
          }}</AppBadge>
        </summary>
        <dl class="modern-services-facts">
          <div>
            <dt>{{ t('official') }}</dt>
            <dd>{{ modality(model.input) }} → {{ modality(model.output) }}</dd>
          </div>
          <div>
            <dt>{{ t('provider') }}</dt>
            <dd>{{ modality(model.provider) }}</dd>
          </div>
          <div v-if="model.query.length">
            <dt>{{ t('query') }}</dt>
            <dd>{{ modality(model.query) }}</dd>
          </div>
          <div v-if="model.documents.length">
            <dt>{{ t('documents') }}</dt>
            <dd>{{ modality(model.documents) }}</dd>
          </div>
          <div v-if="model.source">
            <dt>{{ t('source') }}</dt>
            <dd>
              <a :href="model.source" target="_blank" rel="noreferrer">{{ model.date }}</a>
            </dd>
          </div>
        </dl>
        <h3>{{ t('recent') }}</h3>
        <p v-if="!model.verification.length" class="modern-services-note">
          {{ t('noRecords', { days: query.data.value.windowDays }) }}
        </p>
        <article
          v-for="check in model.verification"
          :key="check.requestId"
          class="modern-services-check"
        >
          <div class="modern-services-heading">
            <span>{{ date(check.at) }} · #{{ check.credentialId }}</span
            ><AppBadge size="xs">{{ t(check.current ? 'current' : 'historical') }}</AppBadge>
          </div>
          <p>
            <AppProtocolTag :protocol="check.format" /> {{ check.inputType }} ·
            {{ check.status === 'success' ? t('completed') : check.status }}
          </p>
          <p v-if="check.input.length">
            {{ modality(check.input)
            }}<template v-if="check.dimensions">
              · {{ t('dimensions', { count: check.dimensions }) }}</template
            >
          </p>
          <p v-if="check.query.length">
            {{ t('query') }} {{ modality(check.query) }} · {{ t('documents') }}
            {{ modality(check.documents) }}
          </p>
          <p>
            {{ t(check.result === 'vector_count_and_shape' ? 'vectorChecked' : 'unchecked') }} ·
            {{ t('quality') }}
          </p>
          <RouterLink :to="{ name: 'modern-logs', query: { request_id: check.requestId } }">{{
            t('viewLog')
          }}</RouterLink>
        </article>
      </details>
    </template>
  </section>
</template>
<style scoped>
.modern-services {
  display: grid;
  align-content: start;
  gap: var(--modern-space-4);
  overflow-y: auto;
  min-height: 0;
  padding-block: var(--modern-space-2);
  padding-right: var(--modern-space-2);
}
.modern-services-heading {
  display: flex;
  align-items: center;
  gap: var(--modern-space-2);
  flex-wrap: wrap;
}
.modern-services h2 {
  font-size: var(--modern-font-size-secondary);
  font-weight: var(--modern-weight-semibold);
  margin: 0;
}
.modern-services h3 {
  font-size: var(--modern-font-size-small);
  font-weight: var(--modern-weight-semibold);
  margin-block: var(--modern-space-4) var(--modern-space-2);
}
.modern-services-routes {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: var(--modern-space-3);
}
.modern-services-route,
.modern-services-model {
  border: var(--modern-line-width) solid var(--modern-border);
  border-radius: var(--modern-radius-control);
  padding: var(--modern-space-4);
  min-width: 0;
}
.modern-services-route p,
.modern-services-check p {
  margin-block: var(--modern-space-2);
  font-size: var(--modern-font-size-small);
  overflow-wrap: anywhere;
}
.modern-services-path {
  display: flex;
  gap: var(--modern-space-2);
  align-items: baseline;
}
.modern-services-path code {
  overflow-wrap: anywhere;
  min-width: 0;
}
.modern-services-path-transparent {
  flex-direction: column;
}
.modern-services-facts {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: var(--modern-space-3);
  margin-block: var(--modern-space-3);
}
.modern-services-facts dt,
.modern-services-note,
.modern-services-limitations {
  color: var(--modern-muted);
  font-size: var(--modern-font-size-small);
}
.modern-services-facts dd {
  margin: 0;
  padding-top: var(--modern-space-1);
  font-size: var(--modern-font-size-small);
  overflow-wrap: anywhere;
}
.modern-services-limitations {
  margin-block: var(--modern-space-3) 0;
  padding-left: var(--modern-space-4);
}
.modern-services-model summary {
  cursor: pointer;
  overflow-wrap: anywhere;
  font-size: var(--modern-font-size-small);
}
.modern-services-model summary code {
  margin-right: var(--modern-space-3);
}
.modern-services-check {
  border-top: var(--modern-line-width) solid var(--modern-border);
  padding-block: var(--modern-space-3);
  font-size: var(--modern-font-size-small);
}
@container modern-group-workspace (max-width:700px) {
  .modern-services {
    overflow-y: visible;
  }
  .modern-services-routes,
  .modern-services-facts {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
