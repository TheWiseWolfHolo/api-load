import assert from "node:assert/strict";
import test from "node:test";
import { hasPrimaryCoarsePointer } from "./pointerMode.ts";

test("hasPrimaryCoarsePointer allows fine primary pointers even when touch is available", () => {
  const env = {
    matchMedia: (query: string) => ({
      matches: query === "(any-pointer: coarse)",
    }),
    navigator: {
      maxTouchPoints: 5,
    },
  };

  assert.equal(hasPrimaryCoarsePointer(env), false);
});

test("hasPrimaryCoarsePointer detects coarse primary pointers", () => {
  const env = {
    matchMedia: (query: string) => ({
      matches: query === "(pointer: coarse)",
    }),
    navigator: {
      maxTouchPoints: 0,
    },
  };

  assert.equal(hasPrimaryCoarsePointer(env), true);
});

test("hasPrimaryCoarsePointer falls back to touch points when media queries are unavailable", () => {
  assert.equal(hasPrimaryCoarsePointer({ navigator: { maxTouchPoints: 1 } }), true);
  assert.equal(hasPrimaryCoarsePointer({ navigator: { maxTouchPoints: 0 } }), false);
});
