<template>
  <div class="key-table">
    <el-card shadow="never">
      <template #header>
        <div class="card-header">
          <span>密钥管理</span>
          <el-button type="primary" @click="handleAddKey" :disabled="!groupStore.selectedGroupId">添加密钥</el-button>
        </div>
      </template>

      <el-table :data="keyStore.keys" v-loading="keyStore.isLoading" style="width: 100%">
        <el-table-column prop="api_key" label="API Key (部分)" min-width="180">
            <template #default="scope">
                {{ scope.row.api_key.substring(0, 3) }}...{{ scope.row.api_key.slice(-4) }}
            </template>
        </el-table-column>
        <el-table-column prop="platform" label="平台" width="100" />
        <el-table-column prop="model_types" label="可用模型" min-width="150">
            <template #default="scope">
                <el-tag v-for="model in scope.row.model_types" :key="model" style="margin-right: 5px;">{{ model }}</el-tag>
            </template>
        </el-table-column>
        <el-table-column prop="rate_limit" label="速率限制" width="120">
            <template #default="scope">
                {{ scope.row.rate_limit }} / {{ scope.row.rate_limit_unit }}
            </template>
        </el-table-column>
        <el-table-column prop="is_active" label="状态" width="80">
            <template #default="scope">
                <el-tag :type="scope.row.is_active ? 'success' : 'danger'">
                    {{ scope.row.is_active ? '启用' : '禁用' }}
                </el-tag>
            </template>
        </el-table-column>
        <el-table-column label="操作" width="150" fixed="right">
          <template #default="scope">
            <el-button size="small" @click="handleEditKey(scope.row)">编辑</el-button>
            <el-popconfirm
                title="确定要删除这个密钥吗？"
                @confirm="handleDeleteKey(scope.row.id)"
            >
                <template #reference>
                    <el-button size="small" type="danger">删除</el-button>
                </template>
            </el-popconfirm>
          </template>
        </el-table-column>
      </el-table>
      <el-empty v-if="!keyStore.isLoading && keyStore.keys.length === 0" description="该分组下暂无密钥"></el-empty>
    </el-card>

    <!-- Add/Edit Dialog -->
    <el-dialog v-model="dialogVisible" :title="dialogTitle" width="50%">
      <el-form :model="keyFormData" label-width="120px" ref="keyFormRef" :rules="keyFormRules">
        <el-form-item label="API Key" prop="api_key">
          <el-input v-model="keyFormData.api_key" placeholder="请输入完整的API Key"></el-input>
        </el-form-item>
        <el-form-item label="平台" prop="platform">
          <el-select v-model="keyFormData.platform" placeholder="请选择平台">
            <el-option label="OpenAI" value="OpenAI"></el-option>
            <el-option label="Gemini" value="Gemini"></el-option>
          </el-select>
        </el-form-item>
        <el-form-item label="可用模型" prop="model_types">
           <el-select
                v-model="keyFormData.model_types"
                multiple
                filterable
                allow-create
                default-first-option
                placeholder="请输入或选择可用模型">
            </el-select>
        </el-form-item>
        <el-form-item label="速率限制" prop="rate_limit">
            <el-input-number v-model="keyFormData.rate_limit" :min="0"></el-input-number>
        </el-form-item>
        <el-form-item label="限制单位" prop="rate_limit_unit">
             <el-select v-model="keyFormData.rate_limit_unit">
                <el-option label="分钟" value="minute"></el-option>
                <el-option label="小时" value="hour"></el-option>
                <el-option label="天" value="day"></el-option>
            </el-select>
        </el-form-item>
        <el-form-item label="启用状态" prop="is_active">
          <el-switch v-model="keyFormData.is_active"></el-switch>
        </el-form-item>
      </el-form>
      <template #footer>
        <span class="dialog-footer">
          <el-button @click="dialogVisible = false">取消</el-button>
          <el-button type="primary" @click="handleConfirmSave" :loading="isSaving">确定</el-button>
        </span>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed } from 'vue';
import { useKeyStore } from '@/stores/keyStore';
import { useGroupStore } from '@/stores/groupStore';
import * as keyApi from '@/api/keys';
import type { Key } from '@/types/models';
import { ElCard, ElTable, ElTableColumn, ElButton, ElTag, ElPopconfirm, ElDialog, ElForm, ElFormItem, ElInput, ElSelect, ElOption, ElSwitch, ElMessage, ElEmpty, ElInputNumber } from 'element-plus';
import type { FormInstance, FormRules } from 'element-plus';

const keyStore = useKeyStore();
const groupStore = useGroupStore();

const dialogVisible = ref(false);
const isSaving = ref(false);
const isEdit = ref(false);
const currentKeyId = ref<string | null>(null);
const keyFormRef = ref<FormInstance>();

const dialogTitle = computed(() => (isEdit.value ? '编辑密钥' : '添加密钥'));

const initialFormData: Omit<Key, 'id' | 'group_id' | 'usage' | 'created_at' | 'updated_at'> = {
    api_key: '',
    platform: 'OpenAI',
    model_types: [],
    rate_limit: 60,
    rate_limit_unit: 'minute',
    is_active: true,
};

const keyFormData = reactive({ ...initialFormData });

const keyFormRules = reactive<FormRules>({
    api_key: [{ required: true, message: '请输入API Key', trigger: 'blur' }],
    platform: [{ required: true, message: '请选择平台', trigger: 'change' }],
    model_types: [{ required: true, message: '请至少输入一个可用模型', trigger: 'change' }],
});


const resetForm = () => {
    Object.assign(keyFormData, initialFormData);
    currentKeyId.value = null;
};

const handleAddKey = () => {
    isEdit.value = false;
    resetForm();
    dialogVisible.value = true;
};

const handleEditKey = (key: Key) => {
    isEdit.value = true;
    resetForm();
    currentKeyId.value = key.id;
    // 只填充表单所需字段
    keyFormData.api_key = key.api_key;
    keyFormData.platform = key.platform;
    keyFormData.model_types = key.model_types;
    keyFormData.rate_limit = key.rate_limit;
    keyFormData.rate_limit_unit = key.rate_limit_unit;
    keyFormData.is_active = key.is_active;
    dialogVisible.value = true;
};

const handleDeleteKey = async (id: string) => {
    try {
        await keyApi.deleteKey(id);
        ElMessage.success('删除成功');
        if (groupStore.selectedGroupId) {
            keyStore.fetchKeys(groupStore.selectedGroupId);
        }
    } catch (error) {
        console.error('Failed to delete key:', error);
        ElMessage.error('删除失败');
    }
};

const handleConfirmSave = async () => {
    if (!keyFormRef.value || !groupStore.selectedGroupId) return;

    try {
        await keyFormRef.value.validate();
        isSaving.value = true;

        const dataToSave = {
            api_key: keyFormData.api_key,
            platform: keyFormData.platform,
            model_types: keyFormData.model_types,
            rate_limit: keyFormData.rate_limit,
            rate_limit_unit: keyFormData.rate_limit_unit,
            is_active: keyFormData.is_active,
        };

        if (isEdit.value && currentKeyId.value) {
            await keyApi.updateKey(currentKeyId.value, dataToSave);
        } else {
            await keyApi.createKey(groupStore.selectedGroupId, dataToSave);
        }

        ElMessage.success('保存成功');
        dialogVisible.value = false;
        await keyStore.fetchKeys(groupStore.selectedGroupId);

    } catch (error) {
        console.error('Failed to save key:', error);
        ElMessage.error('保存失败，请检查表单或查看控制台');
    } finally {
        isSaving.value = false;
    }
};

</script>

<style scoped>
.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.el-table {
    margin-top: 16px;
}
</style>