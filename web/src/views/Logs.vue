<template>
  <div class="logs-view">
    <div class="page-header">
      <h1 class="text-2xl font-semibold text-gray-900">请求日志</h1>
      <p class="mt-2 text-sm text-gray-600">
        查看和管// 筛选器 const filters = reactive({ dateRange: [] as [Date,
        Date] | [], groupId: undefined as number | undefined, statusCode:
        undefined as number | undefined, keyword: '', });志记录
      </p>
    </div>

    <!-- 筛选器 -->
    <div class="filters-card">
      <el-card shadow="never">
        <div class="filters-grid">
          <div class="filter-item">
            <label class="filter-label">时间范围</label>
            <DateRangePicker v-model="filters.dateRange" />
          </div>

          <div class="filter-item">
            <label class="filter-label">分组</label>
            <el-select
              v-model="filters.groupId"
              placeholder="选择分组"
              clearable
              @clear="filters.groupId = ''"
            >
              <el-option
                v-for="group in groupStore.groups"
                :key="group.id"
                :label="group.name"
                :value="group.id"
              />
            </el-select>
          </div>

          <div class="filter-item">
            <label class="filter-label">状态码</label>
            <el-select
              v-model="filters.statusCode"
              placeholder="选择状态码"
              clearable
              @clear="filters.statusCode = ''"
            >
              <el-option label="200 - 成功" :value="200" />
              <el-option label="400 - 请求错误" :value="400" />
              <el-option label="401 - 未授权" :value="401" />
              <el-option label="429 - 限流" :value="429" />
              <el-option label="500 - 服务器错误" :value="500" />
            </el-select>
          </div>

          <div class="filter-item">
            <label class="filter-label">搜索</label>
            <SearchInput
              v-model="filters.keyword"
              placeholder="搜索IP地址或请求路径..."
              @search="handleSearch"
            />
          </div>

          <div class="filter-actions">
            <el-button @click="resetFilters">重置</el-button>
            <el-button type="primary" @click="applyFilters">应用筛选</el-button>
          </div>
        </div>
      </el-card>
    </div>

    <!-- 日志表格 -->
    <div class="logs-table">
      <DataTable
        :data="filteredLogs"
        :columns="tableColumns"
        :loading="loading"
        :pagination="pagination"
        @page-change="handlePageChange"
        @size-change="handleSizeChange"
      >
        <!-- 时间戳列 -->
        <template #timestamp="{ row }">
          <div class="timestamp-cell">
            {{ formatTimestamp(row.timestamp) }}
          </div>
        </template>

        <!-- 状态码列 -->
        <template #status_code="{ row }">
          <el-tag :type="getStatusType(row.status_code)">
            {{ row.status_code }}
          </el-tag>
        </template>

        <!-- 分组列 -->
        <template #group="{ row }">
          <span class="group-name">
            {{ getGroupName(row.group_id) }}
          </span>
        </template>

        <!-- 请求路径列 -->
        <template #request_path="{ row }">
          <el-tooltip placement="top" :content="row.request_path">
            <span class="request-path">
              {{ truncateText(row.request_path, 50) }}
            </span>
          </el-tooltip>
        </template>

        <!-- IP地址列 -->
        <template #source_ip="{ row }">
          <span class="ip-address">{{ row.source_ip }}</span>
        </template>

        <!-- 请求体预览列 -->
        <template #request_body="{ row }">
          <el-button
            size="small"
            text
            @click="showRequestBody(row)"
            v-if="row.request_body_snippet"
          >
            查看详情
          </el-button>
          <span v-else class="text-gray-400">无内容</span>
        </template>
      </DataTable>
    </div>

    <!-- 请求详情对话框 -->
    <el-dialog v-model="detailDialogVisible" title="请求详情" width="800px">
      <div v-if="selectedLog" class="request-detail">
        <div class="detail-section">
          <h4>基本信息</h4>
          <el-descriptions :column="2" border>
            <el-descriptions-item label="时间">
              {{ formatTimestamp(selectedLog.timestamp) }}
            </el-descriptions-item>
            <el-descriptions-item label="状态码">
              <el-tag :type="getStatusType(selectedLog.status_code)">
                {{ selectedLog.status_code }}
              </el-tag>
            </el-descriptions-item>
            <el-descriptions-item label="IP地址">
              {{ selectedLog.source_ip }}
            </el-descriptions-item>
            <el-descriptions-item label="分组">
              {{ getGroupName(selectedLog.group_id) }}
            </el-descriptions-item>
            <el-descriptions-item label="请求路径" :span="2">
              <code>{{ selectedLog.request_path }}</code>
            </el-descriptions-item>
          </el-descriptions>
        </div>

        <div class="detail-section" v-if="selectedLog.request_body_snippet">
          <h4>请求内容</h4>
          <div class="request-body-container">
            <pre class="request-body">{{
              selectedLog.request_body_snippet
            }}</pre>
          </div>
        </div>
      </div>

      <template #footer>
        <el-button @click="detailDialogVisible = false">关闭</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from "vue";
import {
  ElCard,
  ElSelect,
  ElOption,
  ElButton,
  ElTag,
  ElTooltip,
  ElDialog,
  ElDescriptions,
  ElDescriptionsItem,
  ElMessage,
} from "element-plus";
import DataTable from "@/components/common/DataTable.vue";
import SearchInput from "@/components/common/SearchInput.vue";
import DateRangePicker from "@/components/common/DateRangePicker.vue";
import { useGroupStore } from "@/stores/groupStore";
import type { RequestLog } from "@/types/models";

const groupStore = useGroupStore();

const loading = ref(false);
const detailDialogVisible = ref(false);
const selectedLog = ref<RequestLog | null>(null);

// 筛选器
const filters = reactive({
  dateRange: null as [Date, Date] | null,
  groupId: "" as string | number | "",
  statusCode: "" as string | number | "",
  keyword: "",
});

// 分页
const pagination = reactive({
  currentPage: 1,
  pageSize: 20,
  total: 0,
});

// 表格列配置
const tableColumns = [
  { prop: "timestamp", label: "时间", width: 180 },
  { prop: "status_code", label: "状态码", width: 100 },
  { prop: "group", label: "分组", width: 120 },
  { prop: "source_ip", label: "IP地址", width: 140 },
  { prop: "request_path", label: "请求路径", minWidth: 200 },
  { prop: "request_body", label: "请求内容", width: 120 },
];

// 模拟日志数据（实际应该从 logStore 获取）
const mockLogs: RequestLog[] = [
  {
    id: "1",
    timestamp: new Date().toISOString(),
    group_id: 1,
    key_id: 1,
    source_ip: "192.168.1.100",
    status_code: 200,
    request_path: "/v1/chat/completions",
    request_body_snippet:
      '{"model": "gpt-3.5-turbo", "messages": [{"role": "user", "content": "Hello"}]}',
  },
  {
    id: "2",
    timestamp: new Date(Date.now() - 60000).toISOString(),
    group_id: 1,
    key_id: 2,
    source_ip: "192.168.1.101",
    status_code: 429,
    request_path: "/v1/chat/completions",
    request_body_snippet:
      '{"model": "gpt-4", "messages": [{"role": "user", "content": "Hi there"}]}',
  },
  {
    id: "3",
    timestamp: new Date(Date.now() - 120000).toISOString(),
    group_id: 2,
    key_id: 3,
    source_ip: "192.168.1.102",
    status_code: 401,
    request_path: "/v1/models",
    request_body_snippet: "",
  },
];

// 计算属性
const filteredLogs = computed(() => {
  let logs = mockLogs;

  // 应用筛选器
  if (filters.groupId) {
    logs = logs.filter((log) => log.group_id === filters.groupId);
  }

  if (filters.statusCode) {
    logs = logs.filter((log) => log.status_code === filters.statusCode);
  }

  if (filters.keyword) {
    const keyword = filters.keyword.toLowerCase();
    logs = logs.filter(
      (log) =>
        log.source_ip.toLowerCase().includes(keyword) ||
        log.request_path.toLowerCase().includes(keyword)
    );
  }

  if (filters.dateRange) {
    const [start, end] = filters.dateRange;
    logs = logs.filter((log) => {
      const logTime = new Date(log.timestamp);
      return logTime >= start && logTime <= end;
    });
  }

  // 更新分页总数
  pagination.total = logs.length;

  // 应用分页
  const startIndex = (pagination.currentPage - 1) * pagination.pageSize;
  const endIndex = startIndex + pagination.pageSize;

  return logs.slice(startIndex, endIndex);
});

// 方法
const formatTimestamp = (timestamp: string) => {
  return new Date(timestamp).toLocaleString("zh-CN");
};

const getStatusType = (statusCode: number) => {
  if (statusCode >= 200 && statusCode < 300) return "success";
  if (statusCode >= 400 && statusCode < 500) return "warning";
  if (statusCode >= 500) return "danger";
  return "info";
};

const getGroupName = (groupId: number) => {
  const group = groupStore.groups.find((g) => g.id === groupId);
  return group?.name || `分组 ${groupId}`;
};

const truncateText = (text: string, maxLength: number) => {
  return text.length > maxLength ? text.substring(0, maxLength) + "..." : text;
};

const showRequestBody = (log: RequestLog) => {
  selectedLog.value = log;
  detailDialogVisible.value = true;
};

const handleSearch = () => {
  pagination.currentPage = 1;
  // 搜索逻辑已在 computed 中处理
};

const applyFilters = () => {
  pagination.currentPage = 1;
  // 筛选逻辑已在 computed 中处理
  ElMessage.success("筛选条件已应用");
};

const resetFilters = () => {
  filters.dateRange = null;
  filters.groupId = "";
  filters.statusCode = "";
  filters.keyword = "";
  pagination.currentPage = 1;
  ElMessage.success("筛选条件已重置");
};

const handlePageChange = (page: number) => {
  pagination.currentPage = page;
};

const handleSizeChange = (size: number) => {
  pagination.pageSize = size;
  pagination.currentPage = 1;
};

onMounted(() => {
  // 加载分组数据用于筛选
  groupStore.fetchGroups();

  // 加载日志数据
  // TODO: 实现真实的日志加载逻辑
  // logStore.fetchLogs();
});
</script>

<style scoped>
.logs-view {
  padding: 24px;
  background-color: var(--el-bg-color-page);
  min-height: 100vh;
}

.page-header {
  margin-bottom: 24px;
}

.filters-card {
  margin-bottom: 24px;
}

.filters-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 16px;
  align-items: end;
}

.filter-item {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.filter-label {
  font-size: 14px;
  font-weight: 500;
  color: var(--el-text-color-primary);
}

.filter-actions {
  display: flex;
  gap: 8px;
}

.logs-table {
  background-color: white;
  border-radius: 8px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
}

.timestamp-cell {
  font-family: monospace;
  font-size: 13px;
}

.group-name {
  font-weight: 500;
}

.request-path {
  font-family: monospace;
  font-size: 12px;
  color: var(--el-text-color-regular);
}

.ip-address {
  font-family: monospace;
  font-size: 13px;
}

.request-detail {
  max-height: 60vh;
  overflow-y: auto;
}

.detail-section {
  margin-bottom: 24px;
}

.detail-section h4 {
  margin: 0 0 16px 0;
  font-size: 16px;
  font-weight: 600;
  color: var(--el-text-color-primary);
}

.request-body-container {
  background-color: var(--el-bg-color-page);
  border-radius: 6px;
  padding: 16px;
  border: 1px solid var(--el-border-color-light);
}

.request-body {
  margin: 0;
  font-family: "Monaco", "Consolas", monospace;
  font-size: 12px;
  line-height: 1.5;
  color: var(--el-text-color-primary);
  white-space: pre-wrap;
  word-break: break-all;
}

@media (max-width: 768px) {
  .logs-view {
    padding: 16px;
  }

  .filters-grid {
    grid-template-columns: 1fr;
  }

  .filter-actions {
    justify-content: stretch;
  }

  .filter-actions .el-button {
    flex: 1;
  }
}
</style>
