import assert from "node:assert/strict";
import test from "node:test";
import { buildGroupReorderItems } from "./groupOrder.ts";

test("buildGroupReorderItems assigns spaced sort values and returns changed groups only", () => {
  const groups = [
    { id: 7, sort: 30 },
    { id: 2, sort: 10 },
    { id: 5, sort: 20 },
  ];

  const result = buildGroupReorderItems(groups);

  assert.deepEqual(result.items, [
    { id: 7, sort: 10 },
    { id: 2, sort: 20 },
    { id: 5, sort: 30 },
  ]);
  assert.deepEqual(
    result.groups.map(group => ({ id: group.id, sort: group.sort })),
    [
      { id: 7, sort: 10 },
      { id: 2, sort: 20 },
      { id: 5, sort: 30 },
    ]
  );
  assert.deepEqual(groups, [
    { id: 7, sort: 30 },
    { id: 2, sort: 10 },
    { id: 5, sort: 20 },
  ]);
});

test("buildGroupReorderItems returns no payload when order already matches spaced values", () => {
  const result = buildGroupReorderItems([
    { id: 1, sort: 10 },
    { id: 2, sort: 20 },
  ]);

  assert.deepEqual(result.items, []);
  assert.deepEqual(result.groups, [
    { id: 1, sort: 10 },
    { id: 2, sort: 20 },
  ]);
});
