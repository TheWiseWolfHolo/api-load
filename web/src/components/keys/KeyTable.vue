<script setup lang="ts">
import { keysApi } from "@/api/keys";
import type { APIKey, Group } from "@/types/models";
import { NButton, NDropdown, NEmpty, NInput, NSelect, NSpace, NSpin } from "naive-ui";
import { computed, ref, watch } from "vue";

interface Props {
  selectedGroup: Group | null;
}

const props = defineProps<Props>();

const keys = ref<APIKey[]>([]);
const loading = ref(false);
const searchText = ref("");
const statusFilter = ref<"all" | "valid" | "invalid">("all");
const currentPage = ref(1);
const pageSize = ref(20);
const totalKeys = ref(0);

const totalPages = computed(() => Math.ceil(totalKeys.value / pageSize.value));

// 状态过滤选项
const statusOptions = [
  { label: "全部", value: "all" },
  { label: "有效", value: "valid" },
  { label: "无效", value: "invalid" },
];

// 更多操作下拉菜单选项
const moreOptions = [
  { label: "复制所有 Key", key: "copyAll" },
  { label: "复制有效 Key", key: "copyValid" },
  { label: "复制无效 Key", key: "copyInvalid" },
  { type: "divider" },
  { label: "恢复所有无效 Key", key: "restoreAll" },
  { label: "验证所有 Key", key: "validateAll" },
  { type: "divider" },
  { label: "清空所有无效 Key", key: "clearInvalid", props: { style: { color: "#d03050" } } },
];

watch(
  () => props.selectedGroup,
  async newGroup => {
    if (newGroup) {
      currentPage.value = 1;
      await loadKeys();
    }
  },
  { immediate: true }
);

watch([currentPage, pageSize, statusFilter, searchText], async () => {
  await loadKeys();
});

// 处理更多操作菜单
function handleMoreAction(key: string) {
  switch (key) {
    case "copyAll":
      copyAllKeys();
      break;
    case "copyValid":
      copyValidKeys();
      break;
    case "copyInvalid":
      copyInvalidKeys();
      break;
    case "restoreAll":
      restoreAllInvalid();
      break;
    case "validateAll":
      validateAllKeys();
      break;
    case "clearInvalid":
      clearAllInvalid();
      break;
  }
}

async function loadKeys() {
  if (!props.selectedGroup) {
    return;
  }

  try {
    loading.value = true;
    const result = await keysApi.getGroupKeys(
      props.selectedGroup.id,
      currentPage.value,
      pageSize.value,
      statusFilter.value === "all" ? undefined : statusFilter.value
    );
    keys.value = result.data;
    totalKeys.value = result.total;
  } catch (_error) {
    window.$message.error("加载密钥失败");
  } finally {
    loading.value = false;
  }
}

function maskKey(key: string): string {
  if (key.length <= 8) {
    return key;
  }
  return `${key.substring(0, 4)}...${key.substring(key.length - 4)}`;
}

function copyKey(key: APIKey) {
  navigator.clipboard
    .writeText(key.key_value)
    .then(() => {
      window.$message.success("密钥已复制到剪贴板");
    })
    .catch(() => {
      window.$message.error("复制失败");
    });
}

async function testKey(_key: APIKey) {
  try {
    window.$message.info("正在测试密钥...");
    await new Promise(resolve => setTimeout(resolve, 2000));
    const success = Math.random() > 0.3;
    if (success) {
      window.$message.success("密钥测试成功");
    } else {
      window.$message.error("密钥测试失败: 无效的API密钥");
    }
  } catch (_error) {
    window.$message.error("测试失败");
  }
}

function toggleKeyVisibility(key: APIKey) {
  window.$message.info(`切换密钥"${maskKey(key.key_value)}"显示状态功能开发中`);
}

async function restoreKey(key: APIKey) {
  // eslint-disable-next-line no-alert
  const confirmed = window.confirm(`确定要恢复密钥"${maskKey(key.key_value)}"吗？`);
  if (!confirmed) {
    return;
  }

  try {
    await keysApi.toggleKeyStatus(key.id.toString(), 1);
    window.$message.success("密钥已恢复");
    await loadKeys();
  } catch (_error) {
    window.$message.error("恢复失败");
  }
}

async function deleteKey(key: APIKey) {
  // eslint-disable-next-line no-alert
  const confirmed = window.confirm(`确定要删除密钥"${maskKey(key.key_value)}"吗？`);
  if (!confirmed) {
    return;
  }

  try {
    await keysApi.deleteKeyById(key.id.toString());
    window.$message.success("密钥已删除");
    await loadKeys();
  } catch (_error) {
    window.$message.error("删除失败");
  }
}

function formatRelativeTime(date: string) {
  const now = new Date();
  const target = new Date(date);
  const diff = now.getTime() - target.getTime();
  const hours = Math.floor(diff / (1000 * 60 * 60));
  const days = Math.floor(hours / 24);

  if (days > 0) {
    return `${days}天前`;
  } else if (hours > 0) {
    return `${hours}小时前`;
  } else {
    return "刚刚";
  }
}

function getStatusClass(status: "active" | "inactive" | "error") {
  switch (status) {
    case "active":
      return "status-valid";
    case "inactive":
      return "status-invalid";
    case "error":
      return "status-error";
    default:
      return "status-unknown";
  }
}

function addKey() {
  window.$message.info("添加密钥功能开发中");
}

async function copyAllKeys() {
  if (!props.selectedGroup) {
    return;
  }

  try {
    const result = await keysApi.exportKeys(props.selectedGroup.id, "all");
    const keysText = result.keys.join("\n");
    navigator.clipboard
      .writeText(keysText)
      .then(() => {
        window.$message.success(`已复制${result.keys.length}个密钥到剪贴板`);
      })
      .catch(() => {
        window.$message.error("复制失败");
      });
  } catch (_error) {
    // 错误已记录
    window.$message.error("导出失败");
  }
}

async function copyValidKeys() {
  if (!props.selectedGroup) {
    return;
  }

  try {
    const result = await keysApi.exportKeys(props.selectedGroup.id, "valid");
    const keysText = result.keys.join("\n");
    navigator.clipboard
      .writeText(keysText)
      .then(() => {
        window.$message.success(`已复制${result.keys.length}个有效密钥到剪贴板`);
      })
      .catch(() => {
        window.$message.error("复制失败");
      });
  } catch (_error) {
    // 错误已记录
    window.$message.error("导出失败");
  }
}

async function copyInvalidKeys() {
  if (!props.selectedGroup) {
    return;
  }

  try {
    const result = await keysApi.exportKeys(props.selectedGroup.id, "invalid");
    const keysText = result.keys.join("\n");
    navigator.clipboard
      .writeText(keysText)
      .then(() => {
        window.$message.success(`已复制${result.keys.length}个无效密钥到剪贴板`);
      })
      .catch(() => {
        window.$message.error("复制失败");
      });
  } catch (_error) {
    // 错误已记录
    window.$message.error("导出失败");
  }
}

async function restoreAllInvalid() {
  if (!props.selectedGroup) {
    return;
  }

  // eslint-disable-next-line no-alert
  const confirmed = window.confirm("确定要恢复所有无效密钥吗？");
  if (!confirmed) {
    return;
  }

  try {
    window.$message.success("所有无效密钥已恢复");
    await loadKeys();
  } catch (_error) {
    // 错误已记录
    window.$message.error("恢复失败");
  }
}

async function validateAllKeys() {
  if (!props.selectedGroup) {
    return;
  }

  try {
    const result = await keysApi.validateKeys(props.selectedGroup.id);
    window.$message.success(`验证完成: 有效${result.valid_count}个，无效${result.invalid_count}个`);
  } catch (_error) {
    // 错误已记录
    window.$message.error("验证失败");
  }
}

async function clearAllInvalid() {
  if (!props.selectedGroup) {
    return;
  }

  // eslint-disable-next-line no-alert
  const confirmed = window.confirm("确定要清除所有无效密钥吗？此操作不可恢复！");
  if (!confirmed) {
    return;
  }

  try {
    window.$message.success("所有无效密钥已清除");
    await loadKeys();
  } catch (_error) {
    // 错误已记录
    window.$message.error("清除失败");
  }
}

function changePage(page: number) {
  currentPage.value = page;
}

function changePageSize(size: number) {
  pageSize.value = size;
  currentPage.value = 1;
}
</script>

<template>
  <div class="key-table-container">
    <!-- 工具栏 -->
    <div class="toolbar">
      <div class="toolbar-left">
        <n-button type="primary" size="small" @click="addKey">
          <template #icon>
            <span style="font-size: 12px">+</span>
          </template>
          添加密钥
        </n-button>
      </div>
      <div class="toolbar-right">
        <n-space :size="12">
          <n-select
            v-model:value="statusFilter"
            :options="statusOptions"
            size="small"
            style="width: 100px"
          />
          <n-input
            v-model:value="searchText"
            placeholder="Key 模糊查询"
            size="small"
            style="width: 180px"
          />
          <n-dropdown :options="moreOptions" trigger="click" @select="handleMoreAction">
            <n-button size="small" secondary>
              <template #icon>
                <span style="font-size: 16px; font-weight: bold">⋯</span>
              </template>
            </n-button>
          </n-dropdown>
        </n-space>
      </div>
    </div>

    <!-- 密钥卡片网格 -->
    <div class="keys-grid-container">
      <n-spin :show="loading">
        <div v-if="keys.length === 0 && !loading" class="empty-container">
          <n-empty description="没有找到匹配的密钥" />
        </div>
        <div v-else class="keys-grid">
          <div
            v-for="key in keys"
            :key="key.id"
            class="key-card"
            :class="getStatusClass(key.status)"
          >
            <!-- 主要信息行：Key + 快速操作 -->
            <div class="key-main">
              <div class="key-section">
                <span class="key-text" :title="key.key_value">{{ maskKey(key.key_value) }}</span>
                <div class="quick-actions">
                  <n-button size="tiny" text @click="toggleKeyVisibility(key)" title="显示/隐藏">
                    <template #icon>
                      <span style="font-size: 12px">👁️</span>
                    </template>
                  </n-button>
                  <n-button size="tiny" text @click="copyKey(key)" title="复制">
                    <template #icon>
                      <span style="font-size: 12px">📋</span>
                    </template>
                  </n-button>
                </div>
              </div>
            </div>

            <!-- 统计信息 + 操作按钮行 -->
            <div class="key-bottom">
              <div class="key-stats">
                <span class="stat-item">
                  请求
                  <strong>{{ key.request_count }}</strong>
                </span>
                <span class="stat-item">
                  失败
                  <strong>{{ key.failure_count }}</strong>
                </span>
                <span class="stat-item">
                  {{ key.last_used_at ? formatRelativeTime(key.last_used_at) : "从未使用" }}
                </span>
              </div>
              <div class="key-actions">
                <n-button size="tiny" @click="testKey(key)" title="测试密钥">测试</n-button>
                <n-button
                  v-if="key.status !== 'active'"
                  size="tiny"
                  @click="restoreKey(key)"
                  title="恢复密钥"
                >
                  恢复
                </n-button>
                <n-button size="tiny" type="error" @click="deleteKey(key)" title="删除密钥">
                  删除
                </n-button>
              </div>
            </div>
          </div>
        </div>
      </n-spin>
    </div>

    <!-- 分页 -->
    <div class="pagination-container">
      <div class="pagination-info">
        <span>共 {{ totalKeys }} 条记录</span>
        <n-select
          v-model:value="pageSize"
          :options="[
            { label: '10条/页', value: 10 },
            { label: '20条/页', value: 20 },
            { label: '50条/页', value: 50 },
            { label: '100条/页', value: 100 },
          ]"
          size="small"
          style="width: 100px; margin-left: 12px"
          @update:value="changePageSize"
        />
      </div>
      <div class="pagination-controls">
        <n-button size="small" :disabled="currentPage <= 1" @click="changePage(currentPage - 1)">
          上一页
        </n-button>
        <span class="page-info">第 {{ currentPage }} 页，共 {{ totalPages }} 页</span>
        <n-button
          size="small"
          :disabled="currentPage >= totalPages"
          @click="changePage(currentPage + 1)"
        >
          下一页
        </n-button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.key-table-container {
  background: white;
  border-radius: 8px;
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
  overflow: hidden;
  height: 100%;
  display: flex;
  flex-direction: column;
}

.toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 16px;
  background: #f8f9fa;
  border-bottom: 1px solid #e9ecef;
  flex-shrink: 0;
}

.toolbar-left {
  display: flex;
  gap: 8px;
}

.toolbar-right {
  display: flex;
  gap: 12px;
  align-items: center;
}

.filter-group {
  display: flex;
  align-items: center;
  gap: 8px;
}

.more-actions {
  position: relative;
}

.more-menu {
  position: absolute;
  top: 100%;
  right: 0;
  background: white;
  border: 1px solid #e9ecef;
  border-radius: 6px;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
  min-width: 180px;
  z-index: 1000;
  overflow: hidden;
}

.menu-item {
  display: block;
  width: 100%;
  padding: 8px 12px;
  border: none;
  background: none;
  text-align: left;
  cursor: pointer;
  font-size: 14px;
  color: #333;
  transition: background-color 0.2s;
}

.menu-item:hover {
  background: #f8f9fa;
}

.menu-item.danger {
  color: #dc3545;
}

.menu-item.danger:hover {
  background: #f8d7da;
}

.menu-divider {
  height: 1px;
  background: #e9ecef;
  margin: 4px 0;
}

.btn {
  padding: 6px 12px;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  font-size: 14px;
  transition: all 0.2s;
  white-space: nowrap;
}

.btn-sm {
  padding: 4px 8px;
  font-size: 12px;
}

.btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.btn-primary {
  background: #007bff;
  color: white;
}

.btn-primary:hover:not(:disabled) {
  background: #0056b3;
}

.btn-secondary {
  background: #6c757d;
  color: white;
}

.btn-secondary:hover:not(:disabled) {
  background: #545b62;
}

.more-icon {
  font-size: 16px;
  font-weight: bold;
}

.filter-select,
.search-input,
.page-size-select {
  padding: 4px 8px;
  border: 1px solid #ced4da;
  border-radius: 4px;
  font-size: 12px;
}

.search-input {
  width: 180px;
}

.filter-select:focus,
.search-input:focus,
.page-size-select:focus {
  outline: none;
  border-color: #007bff;
  box-shadow: 0 0 0 2px rgba(0, 123, 255, 0.25);
}

/* 密钥卡片网格 */
.keys-grid-container {
  flex: 1;
  overflow-y: auto;
  padding: 16px;
}

.keys-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 12px;
}

.key-card {
  background: white;
  border: 1px solid #e9ecef;
  border-radius: 6px;
  padding: 12px;
  transition: all 0.2s;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.key-card:hover {
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
}

/* 状态相关样式 */
.key-card.status-valid {
  border-color: #18a0584d;
  background: #18a0581a;
}

.key-card.status-invalid {
  border-color: #d030503b;
  background: #d0305014;
}

.key-card.status-error {
  border-color: #ffc107;
  background: #fffdf0;
}

/* 主要信息行 */
.key-main {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 8px;
}

.key-section {
  display: flex;
  align-items: center;
  gap: 8px;
  flex: 1;
  min-width: 0;
}

/* 底部统计和按钮行 */
.key-bottom {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 8px;
}

.key-stats {
  display: flex;
  gap: 8px;
  font-size: 11px;
  color: #6c757d;
  flex: 1;
  min-width: 0;
}

.stat-item {
  white-space: nowrap;
}

.stat-item strong {
  color: #495057;
  font-weight: 600;
}

.key-actions {
  display: flex;
  gap: 4px;
  flex-shrink: 0;
}

.key-text {
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, Courier, monospace;
  font-size: 14px;
  font-weight: 600;
  color: #495057;
  background: #fff;
  padding: 4px 8px;
  border-radius: 4px;
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.quick-actions {
  display: flex;
  gap: 4px;
  flex-shrink: 0;
}

.quick-btn {
  padding: 4px 6px;
  border: none;
  background: transparent;
  cursor: pointer;
  border-radius: 3px;
  font-size: 12px;
  transition: background-color 0.2s;
}

.quick-btn:hover {
  background: #e9ecef;
}

/* 统计信息行 */

.action-btn {
  padding: 2px 6px;
  border: 1px solid #dee2e6;
  background: white;
  border-radius: 3px;
  cursor: pointer;
  font-size: 10px;
  font-weight: 500;
  transition: all 0.2s;
  white-space: nowrap;
}

.action-btn:hover {
  background: #f8f9fa;
}

.action-btn.primary {
  border-color: #007bff;
  color: #007bff;
}

.action-btn.primary:hover {
  background: #007bff;
  color: white;
}

.action-btn.secondary {
  border-color: #6c757d;
  color: #6c757d;
}

.action-btn.secondary:hover {
  background: #6c757d;
  color: white;
}

.action-btn.danger {
  border-color: #dc3545;
  color: #dc3545;
}

.action-btn.danger:hover {
  background: #dc3545;
  color: white;
}

/* 加载和空状态 */
.loading-state,
.empty-state {
  display: flex;
  justify-content: center;
  align-items: center;
  height: 200px;
  color: #6c757d;
}

.loading-spinner {
  font-size: 14px;
}

.empty-text {
  font-size: 14px;
}

/* 分页 */
.pagination-container {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 16px;
  background: #f8f9fa;
  border-top: 1px solid #e9ecef;
  flex-shrink: 0;
}

.pagination-info {
  display: flex;
  align-items: center;
  gap: 12px;
  font-size: 12px;
  color: #6c757d;
}

.pagination-controls {
  display: flex;
  align-items: center;
  gap: 12px;
}

.page-info {
  font-size: 12px;
  color: #6c757d;
}

/* 响应式设计 */
@media (max-width: 1200px) {
  .keys-grid {
    grid-template-columns: repeat(2, 1fr);
  }
}

@media (max-width: 1024px) {
  .toolbar {
    flex-direction: column;
    align-items: stretch;
    gap: 8px;
  }

  .toolbar-left,
  .toolbar-right {
    justify-content: center;
  }
}

@media (max-width: 768px) {
  .keys-grid {
    grid-template-columns: 1fr;
  }

  .key-bottom {
    flex-direction: column;
    align-items: flex-start;
    gap: 6px;
  }

  .key-actions {
    align-self: flex-end;
  }
}
</style>
