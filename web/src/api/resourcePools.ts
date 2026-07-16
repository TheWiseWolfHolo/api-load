import type {
  ResourcePool,
  ResourcePoolInput,
  ResourceStatus,
  UpstreamResource,
  UpstreamResourceInput,
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
    const response = await http.post(`/resource-pools/${id}/resources`, payload);
    return response.data || [];
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
};
