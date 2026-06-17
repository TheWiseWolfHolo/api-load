const CONFIRM_DISABLE_KEY_STORAGE_KEY = "gpt-load-confirm-disable-key";

export function shouldConfirmDisableKey(): boolean {
  return localStorage.getItem(CONFIRM_DISABLE_KEY_STORAGE_KEY) !== "false";
}

export function setShouldConfirmDisableKey(value: boolean): void {
  localStorage.setItem(CONFIRM_DISABLE_KEY_STORAGE_KEY, String(value));
}
