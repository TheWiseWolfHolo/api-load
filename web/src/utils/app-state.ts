import { reactive } from "vue";

interface AppState {
  loading: boolean;
  taskPollingTrigger: number;
}

export const appState = reactive<AppState>({
  loading: false,
  taskPollingTrigger: 0,
});
