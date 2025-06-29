<template>
  <div class="group-config-form">
    <el-card v-if="groupStore.selectedGroupDetails" shadow="never">
      <template #header>
        <div class="card-header">
          <span>分组配置</span>
          <el-button type="primary" @click="handleSave" :loading="isSaving">保存</el-button>
        </div>
      </template>
      <el-form :model="formData" label-width="120px" ref="formRef">
        <el-form-item label="分组名称" prop="name" :rules="[{ required: true, message: '请输入分组名称' }]">
          <el-input v-model="formData.name"></el-input>
        </el-form-item>
        <el-form-item label="描述" prop="description">
          <el-input v-model="formData.description" type="textarea"></el-input>
        </el-form-item>
        <el-form-item label="设为默认" prop="is_default">
          <el-switch v-model="formData.is_default"></el-switch>
        </el-form-item>
      </el-form>
    </el-card>
    <el-empty v-else description="请先从左侧选择一个分组"></el-empty>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, reactive } from 'vue';
import { useGroupStore } from '@/stores/groupStore';
import { updateGroup } from '@/api/groups';
import { ElCard, ElForm, ElFormItem, ElInput, ElButton, ElSwitch, ElMessage, ElEmpty } from 'element-plus';
import type { FormInstance } from 'element-plus';

const groupStore = useGroupStore();
const formRef = ref<FormInstance>();
const isSaving = ref(false);

const formData = reactive({
  name: '',
  description: '',
  is_default: false,
});

watch(() => groupStore.selectedGroupDetails, (newGroup) => {
  if (newGroup) {
    formData.name = newGroup.name;
    formData.description = newGroup.description;
    formData.is_default = newGroup.is_default;
  }
}, { immediate: true, deep: true });

const handleSave = async () => {
  if (!formRef.value || !groupStore.selectedGroupId) return;

  try {
    await formRef.value.validate();
    isSaving.value = true;
    await updateGroup(groupStore.selectedGroupId, {
        name: formData.name,
        description: formData.description,
        is_default: formData.is_default,
    });
    ElMessage.success('保存成功');
    // 刷新列表以获取最新数据
    await groupStore.fetchGroups();
  } catch (error) {
    console.error('Failed to save group config:', error);
    ElMessage.error('保存失败，请查看控制台');
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
</style>