import { reactive } from "vue";

interface AppState {
  loading: boolean;
  taskPollingTrigger: number;
  groupDataRefreshTrigger: number;
  lastCompletedTask?: {
    groupName: string;
    taskType: string;
    finishedAt: string;
  };
}

export const appState = reactive<AppState>({
  loading: false,
  taskPollingTrigger: 0,
  groupDataRefreshTrigger: 0,
});
