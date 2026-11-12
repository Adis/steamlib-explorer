// Which optional table columns are shown, chosen in Settings and remembered
// in this browser. Game is always shown. Hiding a column does not change the
// sort: a hidden column that is the current sort still orders the list.

import { useState } from "react";

export const TABLE_COLUMNS = [
  { id: "playtime", label: "Playtime" },
  { id: "recent", label: "2 weeks" },
  { id: "last_played", label: "Last played" },
  { id: "review", label: "Reviews" },
  { id: "metacritic", label: "Metacritic" },
  { id: "release", label: "Released" },
  { id: "platforms", label: "Platforms" },
] as const;

export type TableColumnId = (typeof TABLE_COLUMNS)[number]["id"];

const KEY = "table.hidden";

// Columns that are empty for most games start hidden.
const DEFAULT_HIDDEN: TableColumnId[] = ["recent", "metacritic"];

function load(): TableColumnId[] {
  try {
    const raw = localStorage.getItem(KEY);
    if (raw === null) return DEFAULT_HIDDEN;
    const v = JSON.parse(raw);
    return Array.isArray(v) ? v.filter((id) => TABLE_COLUMNS.some((c) => c.id === id)) : DEFAULT_HIDDEN;
  } catch {
    return DEFAULT_HIDDEN;
  }
}

/** The hidden table columns, and a setter to show or hide one. */
export function useTableColumns() {
  const [hidden, setHidden] = useState<TableColumnId[]>(load);
  const setShown = (id: TableColumnId, shown: boolean) => {
    const next = shown ? hidden.filter((h) => h !== id) : [...hidden.filter((h) => h !== id), id];
    setHidden(next);
    try {
      localStorage.setItem(KEY, JSON.stringify(next));
    } catch {
      // storage blocked: the choice lasts until reload
    }
  };
  const isShown = (id: TableColumnId) => !hidden.includes(id);
  return { hidden, isShown, setShown };
}
