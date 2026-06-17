import { keysApi } from "./keys";

const updateKeyStatus: (keyId: number, status: "active" | "disabled") => Promise<unknown> =
  keysApi.updateKeyStatus;

void updateKeyStatus;
