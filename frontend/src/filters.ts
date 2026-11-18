// Explorer filters live in the page's query string, so a filtered view can be
// bookmarked, shared and survives a reload. Names match GET /api/games, plus
// `page`, `from_year` and `to_year`, which are translated in apiParams.

import { useCallback, useEffect, useRef, useState } from "react";

export const DEFAULT_LIMIT = 50;

/** Sort used when the address bar has none; must match the backend default. */
export const DEFAULT_SORT = "review";

/**
 * Steam shows no rating below this many reviews ("3 user reviews"), so such
 * games are hidden unless `few_reviews` is set; otherwise a 100% from three
 * reviews tops the Reviews sort.
 */
export const MIN_REVIEWS = 10;

/** Query parameters passed to the backend unchanged. */
const PASS_THROUGH = [
  "q",
  "genre",
  "category",
  "played",
  "min_hours",
  "max_hours",
  "min_review", // kept so older bookmarked links still filter
  "min_review_score", // kept so older links still filter; the panel now sets review_score
  "review_score",
  "min_metacritic",
  "platform",
  "details", // store details state; set by the library panel's counts
  "has_reviews", // set by the library panel's counts
  "sort",
  "order",
  "limit",
];

export type UrlParams = URLSearchParams;
export type SetParam = (key: string, value: string | string[] | null) => void;

export function pageOf(p: UrlParams) {
  return Math.max(1, Math.floor(Number(p.get("page"))) || 1);
}

export function limitOf(p: UrlParams) {
  return Number(p.get("limit")) || DEFAULT_LIMIT;
}

/** The /api/games query for the explorer's URL state. */
export function apiParams(p: UrlParams): URLSearchParams {
  const out = new URLSearchParams();
  for (const [k, v] of p) {
    if (PASS_THROUGH.includes(k) && v.trim() !== "") out.append(k, v);
  }
  if (p.get("few_reviews") !== "1") out.set("min_reviews", String(MIN_REVIEWS));
  // Half-typed years are ignored instead of producing a 400.
  const from = p.get("from_year");
  if (from && /^\d{4}$/.test(from)) out.set("released_after", `${from}-01-01`);
  const to = p.get("to_year");
  if (to && /^\d{4}$/.test(to)) out.set("released_before", `${to}-12-31`);
  const page = pageOf(p);
  if (page > 1) out.set("offset", String((page - 1) * limitOf(p)));
  return out;
}

/** Number of active filters, ignoring sorting and paging. */
export function activeFilterCount(p: UrlParams) {
  const ignore = ["sort", "order", "limit", "page"];
  return [...p.entries()].filter(([k, v]) => !ignore.includes(k) && v !== "")
    .length;
}

/** Parameters kept when a library panel count replaces the filters. */
const VIEW_KEEP = ["sort", "order", "limit"];

/**
 * The explorer link for a library panel count: only these filters, plus
 * games with few reviews, so the list matches the count. Sorting and page
 * size stay as they are.
 */
export function libraryCountHref(filters: Record<string, string>) {
  const current = new URLSearchParams(window.location.search);
  const next = new URLSearchParams();
  for (const k of VIEW_KEEP) {
    const v = current.get(k);
    if (v) next.set(k, v);
  }
  next.set("few_reviews", "1");
  for (const [k, v] of Object.entries(filters)) next.set(k, v);
  return `?${next}`;
}

/**
 * Opens an explorer link without reloading the page: a new history entry
 * (so Back returns to the previous filters), and the explorer picks it up
 * like a Back press.
 */
export function openExplorerLink(href: string) {
  const qs = href.replace(/^\?/, "");
  window.history.pushState(null, "", `${window.location.pathname}?${qs}`);
  saveLastView(qs);
  window.dispatchEvent(new PopStateEvent("popstate"));
}

/** localStorage key holding the explorer's last query string. */
const LAST_VIEW_KEY = "explorer.lastView";

function saveLastView(qs: string) {
  try {
    localStorage.setItem(LAST_VIEW_KEY, qs);
  } catch {
    // Storage blocked: the view just isn't remembered.
  }
}

/**
 * The address bar's parameters, or, when it has none, the sort, filters and
 * page from the last visit, so reopening the app resumes where you left off.
 */
function initialParams(): URLSearchParams {
  const search = window.location.search;
  if (search) {
    saveLastView(search.slice(1)); // an opened link counts as the last view
    return new URLSearchParams(search);
  }
  let saved = "";
  try {
    saved = localStorage.getItem(LAST_VIEW_KEY) ?? "";
  } catch {
    // Storage blocked: start from the defaults.
  }
  if (saved) {
    const { pathname, hash } = window.location;
    window.history.replaceState(null, "", `${pathname}?${saved}${hash}`);
  }
  return new URLSearchParams(saved);
}

/**
 * The page's query string as state. setParam replaces one parameter and goes
 * back to page 1; setPage changes only the page.
 */
export function useUrlParams() {
  const [params, setParams] = useState(initialParams);
  const current = useRef(params);

  useEffect(() => {
    const onPop = () => {
      current.current = new URLSearchParams(window.location.search);
      setParams(current.current);
    };
    window.addEventListener("popstate", onPop);
    return () => window.removeEventListener("popstate", onPop);
  }, []);

  const commit = useCallback((next: URLSearchParams) => {
    current.current = next;
    const qs = next.toString();
    window.history.replaceState(
      null,
      "",
      qs ? `?${qs}` : window.location.pathname,
    );
    saveLastView(qs);
    setParams(next);
  }, []);

  const setParam: SetParam = useCallback(
    (key, value) => {
      const next = new URLSearchParams(current.current);
      next.delete(key);
      next.delete("page");
      for (const v of Array.isArray(value) ? value : value ? [value] : []) {
        next.append(key, v);
      }
      commit(next);
    },
    [commit],
  );

  const setPage = useCallback(
    (page: number) => {
      const next = new URLSearchParams(current.current);
      if (page > 1) next.set("page", String(page));
      else next.delete("page");
      commit(next);
    },
    [commit],
  );

  /** Sets sort and order together. order null means the backend default. */
  const setSort = useCallback(
    (sort: string, order: "asc" | "desc" | null) => {
      const next = new URLSearchParams(current.current);
      next.delete("page");
      if (sort === DEFAULT_SORT) next.delete("sort");
      else next.set("sort", sort);
      if (order) next.set("order", order);
      else next.delete("order");
      commit(next);
    },
    [commit],
  );

  /** Clears every filter but keeps sorting and page size. */
  const clearFilters = useCallback(() => {
    const next = new URLSearchParams();
    for (const k of VIEW_KEEP) {
      const v = current.current.get(k);
      if (v) next.set(k, v);
    }
    commit(next);
  }, [commit]);

  return { params, setParam, setPage, setSort, clearFilters };
}
