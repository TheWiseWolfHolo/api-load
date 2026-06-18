interface PointerEnvironment {
  matchMedia?: (query: string) => { matches: boolean };
  navigator?: {
    maxTouchPoints?: number;
  };
}

export function hasPrimaryCoarsePointer(env: PointerEnvironment): boolean {
  if (typeof env.matchMedia === "function") {
    return env.matchMedia("(pointer: coarse)").matches;
  }
  return Boolean(env.navigator?.maxTouchPoints && env.navigator.maxTouchPoints > 0);
}
