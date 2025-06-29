import apiClient from './index';
import type { Group } from '../types/models';

/**
 * 获取所有分组列表
 */
export const fetchGroups = (): Promise<Group[]> => {
    return apiClient.get('/groups').then(res => res.data.data);
};

/**
 * 获取单个分组的详细信息
 * @param id 分组ID
 */
export const fetchGroup = (id: string): Promise<Group> => {
    return apiClient.get(`/groups/${id}`).then(res => res.data.data);
};

/**
 * 创建一个新的分组
 * @param groupData 新分组的数据
 */
export const createGroup = (groupData: Omit<Group, 'id' | 'created_at' | 'updated_at'>): Promise<Group> => {
    return apiClient.post('/groups', groupData).then(res => res.data.data);
};

/**
 * 更新一个已存在的分组
 * @param id 分组ID
 * @param groupData 要更新的数据
 */
export const updateGroup = (id: string, groupData: Partial<Omit<Group, 'id' | 'created_at' | 'updated_at'>>): Promise<Group> => {
    return apiClient.put(`/groups/${id}`, groupData).then(res => res.data.data);
};

/**
 * 删除一个分组
 * @param id 分组ID
 */
export const deleteGroup = (id: string): Promise<void> => {
    return apiClient.delete(`/groups/${id}`).then(res => res.data);
};