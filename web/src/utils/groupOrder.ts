export interface SortableGroup {
  id?: number;
  sort: number;
}

export interface GroupReorderItem {
  id: number;
  sort: number;
}

export interface GroupReorderResult<T extends SortableGroup> {
  groups: T[];
  items: GroupReorderItem[];
}

export function buildGroupReorderItems<T extends SortableGroup>(
  groups: readonly T[]
): GroupReorderResult<T> {
  const normalizedGroups = groups.map((group, index) => ({
    ...group,
    sort: (index + 1) * 10,
  }));

  const items = normalizedGroups
    .filter((group, index) => group.id && group.sort !== groups[index].sort)
    .map(group => ({
      id: group.id as number,
      sort: group.sort,
    }));

  return {
    groups: normalizedGroups,
    items,
  };
}
