import { reactive } from "vue";

interface AppState {
  loading: boolean;
}

export const appState = reactive<AppState>({
  loading: false,
});
