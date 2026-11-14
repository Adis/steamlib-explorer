// Colour themes. The tokens for each live in index.css under
// :root[data-theme="..."]; the choice is remembered in this browser only.
// index.html applies the saved theme before the app loads, so the page never
// flashes the default colours first.

import { useState } from "react";

// "accessible" themes are listed under Accessibility in Settings.
export const THEMES = [
  { id: "midnight", name: "Midnight", swatch: ["#0a0d14", "#11161f", "#3d9df5"] },
  { id: "amber", name: "Graphite & amber", swatch: ["#0c0c0e", "#141417", "#f5a524"] },
  { id: "violet", name: "Violet", swatch: ["#0d0b13", "#15121d", "#8b7cf6"] },
  { id: "light", name: "Light", swatch: ["#f5f6f8", "#ffffff", "#2563eb"] },
  // Retro DOS file manager look: blue panels, double-line frames, a cyan
  // highlight bar and a function-key bar at the bottom (see index.css).
  { id: "dos", name: "Retro DOS", swatch: ["#0000aa", "#00aaaa", "#ffff55"] },
  // High contrast: at least 7:1 for all text (WCAG AAA), bold edges.
  { id: "hc-dark", name: "High contrast dark", swatch: ["#000000", "#ffffff", "#ffd400"], group: "accessible" },
  { id: "hc-light", name: "High contrast light", swatch: ["#ffffff", "#000000", "#0033aa"], group: "accessible" },
] as const;

export type Theme = (typeof THEMES)[number];

export const isAccessible = (t: Theme) => "group" in t && t.group === "accessible";

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

// Text size: scales the root font size, so text and the spacing around it
// grow together, like browser zoom but for this app only. index.html applies
// the saved size before the app loads.

export const TEXT_SIZES = [100, 125, 150] as const;
export type TextSize = (typeof TEXT_SIZES)[number];

const TEXT_KEY = "textSize";

function savedTextSize(): TextSize {
  try {
    const v = Number(localStorage.getItem(TEXT_KEY));
    return (TEXT_SIZES as readonly number[]).includes(v) ? (v as TextSize) : 100;
  } catch {
    return 100;
  }
}

export function useTextSize() {
  const [size, setSize] = useState<TextSize>(savedTextSize);
  const change = (s: TextSize) => {
    document.documentElement.style.fontSize = s === 100 ? "" : `${s}%`;
    setSize(s);
    try {
      localStorage.setItem(TEXT_KEY, String(s));
    } catch {
      // storage blocked: the size lasts until reload
    }
  };
  return [size, change] as const;
}
