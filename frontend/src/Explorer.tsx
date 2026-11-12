import { useState } from "react";
import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { fetchGames } from "./api.ts";
import FilterPanel from "./FilterPanel.tsx";
import GamesTable from "./GamesTable.tsx";
import {
  activeFilterCount,
  apiParams,
  DEFAULT_LIMIT,
  limitOf,
  pageOf,
  useUrlParams,
} from "./filters.ts";

export default function Explorer() {
  const { params, setParam, setPage, setSort, clearFilters } = useUrlParams();
  const [filtersOpen, setFiltersOpen] = useState(false);
  const filterCount = activeFilterCount(params);
  const query = apiParams(params);
  const games = useQuery({
    queryKey: ["games", query.toString()],
    queryFn: () => fetchGames(query),
    // Keep the old page on screen while the next one loads.
    placeholderData: keepPreviousData,
  });

  const sort = params.get("sort") || "name";
  const order = params.get("order");
  const descending = order ? order === "desc" : sort !== "name";
  const page = pageOf(params);
  const limit = limitOf(params);
  const data = games.data;
  const pages = data ? Math.max(1, Math.ceil(data.total / limit)) : 1;

  return (
    <div className="flex flex-col gap-6 lg:flex-row">
      <div className="lg:sticky lg:top-20 lg:max-h-[calc(100vh-6rem)] lg:w-60 lg:shrink-0 lg:overflow-y-auto lg:pr-3 sidebar-scroll">
        {/* On narrow screens the filters fold away above the results. */}
        <button
          onClick={() => setFiltersOpen(!filtersOpen)}
          aria-expanded={filtersOpen}
          className="w-full rounded-md border border-line-strong px-3 py-2 text-left text-sm hover:bg-raised lg:hidden"
        >
          {filtersOpen ? "Hide filters" : "Show filters"}
          {filterCount > 0 && ` (${filterCount} active)`}
        </button>
        <div className={`${filtersOpen ? "mt-4 block" : "hidden"} lg:mt-0 lg:block`}>
          <FilterPanel params={params} setParam={setParam} clearFilters={clearFilters} />
        </div>
      </div>

      <section className="min-w-0 flex-1 space-y-3">
        <div className="flex flex-wrap items-center justify-between gap-3 text-sm">
          <p className="font-medium text-fg tabular-nums" aria-live="polite">
            {data
              ? `${data.total.toLocaleString()} ${data.total === 1 ? "game" : "games"}`
              : "Loading games…"}
            {games.isFetching && data && <span className="ml-2 text-faint">updating…</span>}
          </p>
          <label className="flex items-center gap-2 text-muted">
            Per page
            <select
              value={String(limit)}
              onChange={(e) =>
                setParam("limit", e.target.value === String(DEFAULT_LIMIT) ? null : e.target.value)
              }
              className="rounded-md border border-line-strong bg-surface px-2 py-1 text-fg hover:border-faint focus:border-accent focus:outline-none"
            >
              {[25, 50, 100, 200].map((n) => (
                <option key={n}>{n}</option>
              ))}
            </select>
          </label>
        </div>

        {games.isError && (
          <div role="alert" className="rounded-md border border-amber-700 bg-amber-950 px-4 py-3 text-sm text-amber-200">
            Could not load games: {games.error.message}
          </div>
        )}

        {data && data.games.length > 0 && (
          <GamesTable games={data.games} sort={sort} descending={descending} onSort={setSort} />
        )}

        {data && data.games.length === 0 && (
          <div className="rounded-xl border border-dashed border-line-strong bg-surface/50 p-10 text-center text-muted">
            {filterCount > 0 ? (
              <>
                No games match these filters.{" "}
                <button onClick={clearFilters} className="text-link hover:text-link-hover">
                  Clear filters
                </button>
              </>
            ) : page > 1 ? (
              <>
                This page is empty.{" "}
                <button onClick={() => setPage(1)} className="text-link hover:text-link-hover">
                  Go to the first page
                </button>
              </>
            ) : (
              "Your library is empty. Press Sync library to load your games."
            )}
          </div>
        )}

        {data && data.total > limit && (
          <nav className="flex items-center justify-between text-sm" aria-label="Pages">
            <button
              onClick={() => setPage(page - 1)}
              disabled={page <= 1}
              className="rounded-md border border-line-strong px-3 py-1.5 hover:bg-raised disabled:opacity-40"
            >
              Previous
            </button>
            <span className="text-muted">
              {(data.offset + 1).toLocaleString()}–
              {Math.min(data.offset + limit, data.total).toLocaleString()} of{" "}
              {data.total.toLocaleString()} · page {page} of {pages}
            </span>
            <button
              onClick={() => setPage(page + 1)}
              disabled={page >= pages}
              className="rounded-md border border-line-strong px-3 py-1.5 hover:bg-raised disabled:opacity-40"
            >
              Next
            </button>
          </nav>
        )}
      </section>
    </div>
  );
}
