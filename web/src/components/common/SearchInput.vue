<template>
  <el-autocomplete
    v-model="query"
    :fetch-suggestions="fetchSuggestions"
    placeholder="请输入搜索内容"
    clearable
    @select="handleSelect"
    @input="handleInput"
    class="search-input"
  >
    <template #prepend>
      <el-icon><Search /></el-icon>
    </template>
  </el-autocomplete>
</template>

<script setup lang="ts">
import { ref, defineEmits, defineProps, withDefaults } from 'vue';
import { ElAutocomplete, ElIcon } from 'element-plus';
import { Search } from '@element-plus/icons-vue';
import { debounce } from 'lodash-es';

interface Suggestion {
  value: string;
  [key: string]: any;
}

const props = withDefaults(defineProps<{
  modelValue: string;
  suggestions?: (queryString: string) => Promise<Suggestion[]> | Suggestion[];
  debounceTime?: number;
}>(), {
  suggestions: () => [],
  debounceTime: 300,
});

const emit = defineEmits<{
  (e: 'update:modelValue', value: string): void;
  (e: 'search', value: string): void;
  (e: 'select', item: Suggestion): void;
}>();

const query = ref(props.modelValue);

const fetchSuggestions = (queryString: string, cb: (suggestions: Suggestion[]) => void) => {
  const results = props.suggestions(queryString);
  if (results instanceof Promise) {
    results.then(cb);
  } else {
    cb(results);
  }
};

const debouncedSearch = debounce((value: string) => {
  emit('search', value);
}, props.debounceTime);

const handleInput = (value: string) => {
  query.value = value;
  emit('update:modelValue', value);
  debouncedSearch(value);
};

const handleSelect = (item: Record<string, any>) => {
  const suggestion = item as Suggestion;
  emit('select', suggestion);
  emit('search', suggestion.value);
};
</script>

<style scoped>
.search-input {
  width: 100%;
}
</style>