<template>
  <div class="data-table-container">
    <el-table
      v-loading="loading"
      :data="data"
      style="width: 100%"
      @selection-change="handleSelectionChange"
      @sort-change="handleSortChange"
    >
      <el-table-column v-if="selectable" type="selection" width="55" />
      <el-table-column
        v-for="column in columns"
        :key="column.prop"
        :prop="column.prop"
        :label="column.label"
        :width="column.width"
        :sortable="column.sortable ? 'custom' : false"
      >
        <template #default="{ row }">
          <slot :name="column.prop" :row="row">
            {{ row[column.prop] }}
          </slot>
        </template>
      </el-table-column>
      <el-table-column v-if="$slots.actions" label="操作" fixed="right" width="180">
        <template #default="{ row }">
          <slot name="actions" :row="row"></slot>
        </template>
      </el-table-column>
    </el-table>

    <el-pagination
      v-if="pagination"
      class="pagination"
      :current-page="pagination.currentPage"
      :page-size="pagination.pageSize"
      :total="pagination.total"
      :page-sizes="[10, 20, 50, 100]"
      layout="total, sizes, prev, pager, next, jumper"
      @current-change="handlePageChange"
      @size-change="handleSizeChange"
    />
  </div>
</template>

<script setup lang="ts">
import { defineProps, withDefaults, defineEmits } from 'vue';
import { ElTable, ElTableColumn, ElPagination, ElLoadingDirective as vLoading } from 'element-plus';

export interface TableColumn {
  prop: string;
  label: string;
  width?: string | number;
  sortable?: boolean;
}

export interface PaginationConfig {
  currentPage: number;
  pageSize: number;
  total: number;
}

withDefaults(defineProps<{
  data: any[];
  columns: TableColumn[];
  loading?: boolean;
  selectable?: boolean;
  pagination?: PaginationConfig;
}>(), {
  loading: false,
  selectable: false,
  pagination: undefined,
});

const emit = defineEmits<{
  (e: 'selection-change', selection: any[]): void;
  (e: 'sort-change', { column, prop, order }: { column: any; prop: string; order: string | null }): void;
  (e: 'page-change', page: number): void;
  (e: 'size-change', size: number): void;
}>();

const handleSelectionChange = (selection: any[]) => {
  emit('selection-change', selection);
};

const handleSortChange = ({ column, prop, order }: { column: any; prop: string; order: string | null }) => {
  emit('sort-change', { column, prop, order });
};

const handlePageChange = (page: number) => {
  emit('page-change', page);
};

const handleSizeChange = (size: number) => {
  emit('size-change', size);
};
</script>

<style scoped>
.data-table-container {
  width: 100%;
}
.pagination {
  margin-top: 20px;
  display: flex;
  justify-content: flex-end;
}
</style>