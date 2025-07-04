<script setup lang="ts">
import { keysApi } from "@/api/keys";
import type { APIKey, Group } from "@/types/models";
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
const showMoreMenu = ref(false);

const totalPages = computed(() => Math.ceil(totalKeys.value / pageSize.value));

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
  } catch (error) {
    console.error("加载密钥失败:", error);
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
    // TODO: 实现密钥测试 API
    await new Promise(resolve => setTimeout(resolve, 2000));
    const success = Math.random() > 0.3; // 模拟测试结果
    if (success) {
      window.$message.success("密钥测试成功");
    } else {
      window.$message.error("密钥测试失败: 无效的API密钥");
    }
  } catch (error) {
    console.error("测试密钥失败:", error);
    window.$message.error("测试失败");
  }
}

function toggleKeyVisibility(key: APIKey) {
  // TODO: 实现密钥显示/隐藏切换
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
  } catch (error) {
    console.error("恢复密钥失败:", error);
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
  } catch (error) {
    console.error("删除密钥失败:", error);
    window.$message.error("删除失败");
  }
}

function formatDate(date: string) {
  return new Date(date).toLocaleDateString();
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

function getStatusText(status: "active" | "inactive" | "error") {
  switch (status) {
    case "active":
      return "有效";
    case "inactive":
      return "无效";
    case "error":
      return "错误";
    default:
      return "未知";
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
  } catch (error) {
    console.error("导出失败:", error);
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
  } catch (error) {
    console.error("导出失败:", error);
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
  } catch (error) {
    console.error("导出失败:", error);
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
    // TODO: 实现恢复所有无效密钥 API
    window.$message.success("所有无效密钥已恢复");
    await loadKeys();
  } catch (error) {
    console.error("恢复失败:", error);
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
  } catch (error) {
    console.error("验证失败:", error);
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
    // TODO: 实现清除所有无效密钥 API
    window.$message.success("所有无效密钥已清除");
    await loadKeys();
  } catch (error) {
    console.error("清除失败:", error);
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
        <button @click="addKey" class="btn btn-primary btn-sm">+ 添加密钥</button>
      </div>
      <div class="toolbar-right">
        <div class="filter-group">
          <select v-model="statusFilter" class="filter-select">
            <option value="all">全部</option>
            <option value="valid">有效</option>
            <option value="invalid">无效</option>
          </select>
        </div>
        <div class="filter-group">
          <input v-model="searchText" type="text" placeholder="Key 模糊查询" class="search-input" />
        </div>
        <div class="more-actions">
          <button @click="showMoreMenu = !showMoreMenu" class="btn btn-secondary btn-sm">
            <span class="more-icon">⋯</span>
          </button>
          <div v-if="showMoreMenu" class="more-menu">
            <button @click="copyAllKeys" class="menu-item">复制所有 Key</button>
            <button @click="copyValidKeys" class="menu-item">复制有效 Key</button>
            <button @click="copyInvalidKeys" class="menu-item">复制无效 Key</button>
            <div class="menu-divider" />
            <button @click="restoreAllInvalid" class="menu-item">恢复所有无效 Key</button>
            <button @click="validateAllKeys" class="menu-item">验证所有 Key</button>
            <div class="menu-divider" />
            <button @click="clearAllInvalid" class="menu-item danger">清空所有无效 Key</button>
          </div>
        </div>
      </div>
    </div>

    <!-- 密钥表格 -->
    <div class="table-container">
      <table class="key-table">
        <thead>
          <tr>
            <th class="key-column">密钥 (Key)</th>
            <th class="status-column">状态</th>
            <th class="usage-column">24小时请求</th>
            <th class="last-used-column">最后使用</th>
            <th class="created-column">创建时间</th>
            <th class="actions-column">操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="loading" class="loading-row">
            <td colspan="6" class="loading-cell">
              <div class="loading-spinner">加载中...</div>
            </td>
          </tr>
          <tr v-else-if="keys.length === 0" class="empty-row">
            <td colspan="6" class="empty-cell">
              <div class="empty-text">没有找到匹配的密钥</div>
            </td>
          </tr>
          <tr v-else v-for="key in keys" :key="key.id" class="key-row">
            <td class="key-column">
              <div class="key-content">
                <span class="key-text" :title="key.key_value">{{ maskKey(key.key_value) }}</span>
                <div class="key-actions">
                  <button @click="copyKey(key)" class="key-btn" title="复制">
                    <span class="icon">📋</span>
                  </button>
                  <button @click="toggleKeyVisibility(key)" class="key-btn" title="显示/隐藏">
                    <span class="icon">👁️</span>
                  </button>
                </div>
              </div>
            </td>
            <td class="status-column">
              <span :class="['status-badge', getStatusClass(key.status)]">
                {{ getStatusText(key.status) }}
              </span>
            </td>
            <td class="usage-column">
              <span class="usage-text">{{ key.request_count }} / {{ key.failure_count }}</span>
            </td>
            <td class="last-used-column">
              <span class="time-text">
                {{ key.last_used_at ? formatRelativeTime(key.last_used_at) : "从未使用" }}
              </span>
            </td>
            <td class="created-column">
              <span class="time-text">{{ formatDate(key.created_at) }}</span>
            </td>
            <td class="actions-column">
              <div class="action-buttons">
                <button @click="copyKey(key)" class="action-btn" title="复制">复制</button>
                <button @click="testKey(key)" class="action-btn" title="测试">测试</button>
                <button
                  v-if="key.status !== 'active'"
                  @click="restoreKey(key)"
                  class="action-btn"
                  title="恢复"
                >
                  恢复
                </button>
                <button @click="deleteKey(key)" class="action-btn danger" title="删除">删除</button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- 分页 -->
    <div class="pagination-container">
      <div class="pagination-info">
        <span>共 {{ totalKeys }} 条记录</span>
        <select v-model="pageSize" @change="changePageSize(pageSize)" class="page-size-select">
          <option :value="10">10条/页</option>
          <option :value="20">20条/页</option>
          <option :value="50">50条/页</option>
          <option :value="100">100条/页</option>
        </select>
      </div>
      <div class="pagination-controls">
        <button
          @click="changePage(currentPage - 1)"
          :disabled="currentPage <= 1"
          class="btn btn-secondary btn-sm"
        >
          上一页
        </button>
        <span class="page-info">第 {{ currentPage }} 页，共 {{ totalPages }} 页</span>
        <button
          @click="changePage(currentPage + 1)"
          :disabled="currentPage >= totalPages"
          class="btn btn-secondary btn-sm"
        >
          下一页
        </button>
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

.table-container {
  flex: 1;
  overflow-y: auto;
}

.key-table {
  width: 100%;
  border-collapse: collapse;
  background: white;
  font-size: 13px;
}

.key-table th,
.key-table td {
  padding: 8px 12px;
  text-align: left;
  border-bottom: 1px solid #e9ecef;
  vertical-align: middle;
}

.key-table th {
  background: #f8f9fa;
  font-weight: 600;
  color: #495057;
  font-size: 12px;
  position: sticky;
  top: 0;
  z-index: 10;
}

.key-column {
  width: 35%;
}

.status-column {
  width: 10%;
}

.usage-column {
  width: 15%;
}

.last-used-column {
  width: 15%;
}

.created-column {
  width: 15%;
}

.actions-column {
  width: 10%;
}

.key-content {
  display: flex;
  align-items: center;
  gap: 8px;
}

.key-text {
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, Courier, monospace;
  font-size: 12px;
  color: #495057;
  background: #f8f9fa;
  padding: 2px 6px;
  border-radius: 3px;
  flex: 1;
  min-width: 0;
}

.key-actions {
  display: flex;
  gap: 2px;
  flex-shrink: 0;
}

.key-btn {
  padding: 2px 4px;
  border: none;
  background: transparent;
  cursor: pointer;
  border-radius: 3px;
  transition: background-color 0.2s;
}

.key-btn:hover {
  background: #e9ecef;
}

.key-btn .icon {
  font-size: 12px;
}

.status-badge {
  display: inline-block;
  padding: 2px 6px;
  border-radius: 10px;
  font-size: 10px;
  font-weight: 600;
  text-align: center;
  min-width: 40px;
}

.status-valid {
  background: #d4edda;
  color: #155724;
}

.status-invalid {
  background: #f8d7da;
  color: #721c24;
}

.status-error {
  background: #fff3cd;
  color: #856404;
}

.status-unknown {
  background: #d1ecf1;
  color: #0c5460;
}

.usage-text {
  font-weight: 500;
  color: #495057;
  font-size: 12px;
}

.time-text {
  font-size: 11px;
  color: #6c757d;
}

.action-buttons {
  display: flex;
  gap: 2px;
  flex-wrap: nowrap;
}

.action-btn {
  padding: 2px 6px;
  border: none;
  border-radius: 3px;
  cursor: pointer;
  font-size: 10px;
  transition: all 0.2s;
  white-space: nowrap;
  background: #f8f9fa;
  color: #495057;
  border: 1px solid #dee2e6;
}

.action-btn:hover {
  background: #e9ecef;
  border-color: #adb5bd;
}

.action-btn.danger {
  color: #dc3545;
}

.action-btn.danger:hover {
  background: #f8d7da;
  border-color: #dc3545;
}

.loading-row,
.empty-row {
  height: 80px;
}

.loading-cell,
.empty-cell {
  text-align: center;
  vertical-align: middle;
  color: #6c757d;
}

.loading-spinner {
  font-size: 14px;
}

.empty-text {
  font-size: 14px;
}

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

  .action-buttons {
    flex-direction: column;
    gap: 1px;
  }

  .action-btn {
    font-size: 9px;
    padding: 1px 4px;
  }
}
</style>
