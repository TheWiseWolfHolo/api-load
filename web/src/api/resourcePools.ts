import type {
  BulkResourceDeleteResult,
  BulkResourceStatusResult,
  CredentialBatchUpdateInput,
  ResourceListParams,
  ResourceListResponse,
  ResourcePool,
  ResourcePoolInput,
  ResourceStatus,
  ResourceValidationGroup,
  ResourceValidationResult,
  UpstreamResource,
  UpstreamResourceInput,
  UpstreamResourceUpdateInput,
} from "@/types/models";
import http from "@/utils/http";

export const resourcePoolsApi = {
  async listPools(): Promise<ResourcePool[]> {
    const response = await http.get("/resource-pools");
    return response.data || [];
  },

  async createPool(payload: ResourcePoolInput): Promise<ResourcePool> {
    const response = await http.post("/resource-pools", payload);
    return response.data;
  },

  async updatePool(id: number, payload: Partial<ResourcePoolInput>): Promise<ResourcePool> {
    const response = await http.put(`/resource-pools/${id}`, payload);
    return response.data;
  },

  deletePool(id: number): Promise<void> {
    return http.delete(`/resource-pools/${id}`);
  },

  async addResources(id: number, payload: UpstreamResourceInput[]): Promise<UpstreamResource[]> {
    const response = await http.post(`/resource-pools/${id}/resources`, payload, {
      hideMessage: true,
    });
    return response.data || [];
  },

  async listResources(id: number, params: ResourceListParams): Promise<ResourceListResponse> {
    const response = await http.get(`/resource-pools/${id}/resources`, { params });
    return response.data;
  },

  async listValidationGroups(poolId: number): Promise<ResourceValidationGroup[]> {
    const response = await http.get(`/resource-pools/${poolId}/validation-groups`);
    return response.data || [];
  },

  async testResource(
    poolId: number,
    resourceId: number,
    groupId: number
  ): Promise<ResourceValidationResult> {
    const response = await http.post(
      `/resource-pools/${poolId}/resources/${resourceId}/test`,
      { group_id: groupId },
      { hideMessage: true }
    );
    return response.data;
  },

  async updateResource(
    poolId: number,
    resourceId: number,
    payload: UpstreamResourceUpdateInput
  ): Promise<UpstreamResource> {
    const response = await http.put(`/resource-pools/${poolId}/resources/${resourceId}`, payload);
    return response.data;
  },

  async updateResourceStatus(
    poolId: number,
    resourceId: number,
    status: Extract<ResourceStatus, "active" | "disabled">
  ): Promise<UpstreamResource> {
    const response = await http.put(`/resource-pools/${poolId}/resources/${resourceId}/status`, {
      status,
    });
    return response.data;
  },

  deleteResource(poolId: number, resourceId: number): Promise<void> {
    return http.delete(`/resource-pools/${poolId}/resources/${resourceId}`);
  },

  async bulkUpdateResourceStatus(
    poolId: number,
    resourceIds: number[],
    status: Extract<ResourceStatus, "active" | "disabled">
  ): Promise<BulkResourceStatusResult> {
    const response = await http.post(`/resource-pools/${poolId}/resources/batch-status`, {
      resource_ids: resourceIds,
      status,
    });
    return response.data;
  },

  async bulkUpdateResources(
    poolId: number,
    resourceIds: number[],
    payload: CredentialBatchUpdateInput
  ): Promise<BulkResourceStatusResult> {
    const response = await http.post(`/resource-pools/${poolId}/resources/batch-update`, {
      resource_ids: resourceIds,
      ...payload,
    });
    return response.data;
  },

  async bulkDeleteResources(
    poolId: number,
    payload: { resource_ids?: number[]; keys?: string[] }
  ): Promise<BulkResourceDeleteResult> {
    const response = await http.post(`/resource-pools/${poolId}/resources/batch-delete`, payload);
    return response.data;
  },

  async importResources(poolId: number, content: string): Promise<UpstreamResource[]> {
    const response = await http.post(
      `/resource-pools/${poolId}/resources/import`,
      { content },
      { hideMessage: true }
    );
    return response.data || [];
  },

  exportResources(
    poolId: number,
    options: {
      content: "full" | "keys";
      format: "jsonl" | "csv" | "txt";
      status?: "all" | "active" | "cooling" | "invalid" | "disabled";
      enabled?: boolean;
    }
  ): void {
    const authKey = localStorage.getItem("authKey");
    if (!authKey) {
      return;
    }
    const params = new URLSearchParams({
      key: authKey,
      content: options.content,
      format: options.format,
      status: options.status ?? "all",
    });
    if (options.enabled !== undefined) {
      params.set("enabled", String(options.enabled));
    }
    const link = document.createElement("a");
    link.href = `${http.defaults.baseURL}/resource-pools/${poolId}/resources/export?${params}`;
    link.download = `resource-pool-${poolId}-${options.content}-${options.status ?? "all"}-${Date.now()}.${options.format}`;
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
  },
};
