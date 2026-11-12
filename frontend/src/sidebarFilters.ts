// Which filter groups the sidebar shows, chosen in Settings and remembered in
// this browser. Hiding a group only hides its controls: a value already in
// the URL still applies, and "Clear all" still clears it.

import { useState } from "react";

export const SIDEBAR_FILTERS = [
  { id: "search", label: "Search by name" },
  { id: "played", label: "Played" },
  { id: "playtime", label: "Playtime" },
  { id: "reviews", label: "Steam reviews" },
  { id: "metacritic", label: "Metacritic" },
  { id: "year", label: "Release year" },
  { id: "platforms", label: "Platforms" },
  { id: "genres", label: "Genres" },
  { id: "features", label: "Features" },
] as const;

export type SidebarFilterId = (typeof SIDEBAR_FILTERS)[number]["id"];

const KEY = "sidebar.hidden";

function load(): SidebarFilterId[] {
  try {
    const v = JSON.parse(localStorage.getItem(KEY) ?? "[]");
    return Array.isArray(v) ? v.filter((id) => SIDEBAR_FILTERS.some((f) => f.id === id)) : [];
  } catch {
    return [];
  }
}

/** The hidden filter groups, and a setter to show or hide one. */
export function useSidebarFilters() {
  const [hidden, setHidden] = useState<SidebarFilterId[]>(load);
  const setShown = (id: SidebarFilterId, shown: boolean) => {
    const next = shown ? hidden.filter((h) => h !== id) : [...hidden.filter((h) => h !== id), id];
    setHidden(next);
    try {
      localStorage.setItem(KEY, JSON.stringify(next));
    } catch {
      // storage blocked: the choice lasts until reload
    }
  };
  const isShown = (id: SidebarFilterId) => !hidden.includes(id);
  return { isShown, setShown };
}
