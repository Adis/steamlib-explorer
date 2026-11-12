// Colour themes. The tokens for each live in index.css under
// :root[data-theme="..."]; the choice is remembered in this browser only.
// index.html applies the saved theme before the app loads, so the page never
// flashes the default colours first.

import { useState } from "react";

export const THEMES = [
  { id: "midnight", name: "Midnight", swatch: ["#0a0d14", "#11161f", "#3d9df5"] },
  { id: "amber", name: "Graphite & amber", swatch: ["#0c0c0e", "#141417", "#f5a524"] },
  { id: "violet", name: "Violet", swatch: ["#0d0b13", "#15121d", "#8b7cf6"] },
  { id: "light", name: "Light", swatch: ["#f5f6f8", "#ffffff", "#2563eb"] },
] as const;

export type ThemeId = (typeof THEMES)[number]["id"];

const KEY = "theme";
const DEFAULT: ThemeId = "midnight";

function saved(): ThemeId {
  try {
    const v = localStorage.getItem(KEY);
    return THEMES.some((t) => t.id === v) ? (v as ThemeId) : DEFAULT;
  } catch {
    return DEFAULT;
  }
}

function apply(id: ThemeId) {
  if (id === DEFAULT) document.documentElement.removeAttribute("data-theme");
  else document.documentElement.dataset.theme = id;
}

export function useTheme() {
  const [theme, setTheme] = useState<ThemeId>(saved);
  const change = (id: ThemeId) => {
    apply(id);
    setTheme(id);
    try {
      localStorage.setItem(KEY, id);
    } catch {
      // storage blocked: the theme lasts until reload
    }
  };
  return [theme, change] as const;
}
