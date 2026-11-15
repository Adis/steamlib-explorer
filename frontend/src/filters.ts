// Explorer filters live in the page's query string, so a filtered view can be
// bookmarked, shared and survives a reload. Names match GET /api/games, plus
// `page`, `from_year` and `to_year`, which are translated in apiParams.

import { useCallback, useEffect, useRef, useState } from "react";

export const DEFAULT_LIMIT = 50;

/** Sort used when the address bar has none; must match the backend default. */
export const DEFAULT_SORT = "review";

/** Query parameters passed to the backend unchanged. */
const PASS_THROUGH = [
  "q",
  "genre",
  "category",
  "played",
  "min_hours",
  "max_hours",
  "min_review", // kept so older bookmarked links still filter
  "min_review_score",
  "min_metacritic",
  "platform",
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

/**
 * The page's query string as state. setParam replaces one parameter and goes
 * back to page 1; setPage changes only the page.
 */
export function useUrlParams() {
  const [params, setParams] = useState(
    () => new URLSearchParams(window.location.search),
  );
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
    for (const k of ["sort", "order", "limit"]) {
      const v = current.current.get(k);
      if (v) next.set(k, v);
    }
    commit(next);
  }, [commit]);

  return { params, setParam, setPage, setSort, clearFilters };
}
