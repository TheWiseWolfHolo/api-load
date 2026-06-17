import {
  setShouldConfirmDisableKey,
  shouldConfirmDisableKey,
} from "./preferences";

const currentValue: boolean = shouldConfirmDisableKey();
setShouldConfirmDisableKey(currentValue);
