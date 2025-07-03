import http from "@/utils/http";

export interface Setting {
  key: string;
  name: string;
  value: string | number;
  type: "int" | "string";
  min_value?: number;
  description: string;
}

export interface SettingCategory {
  category_name: string;
  settings: Setting[];
}

export type SettingsUpdatePayload = Record<string, string | number>;

export const settingsApi = {
  getSettings: async (): Promise<SettingCategory[]> => {
    const response = await http.get("/settings");
    return response.data || [];
  },
  updateSettings: (data: SettingsUpdatePayload): Promise<void> => {
    return http.put("/settings", data);
  },
};
