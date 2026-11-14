import { useEffect } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ApiError, fetchSyncStatus, startSync, type SyncRun, type SyncStatus } from "./api.ts";
import { ErrorBox, formatDateTime } from "./ui.tsx";

/** Library status, the Sync button, and progress while a sync runs. */
export default function SyncPanel() {
  const queryClient = useQueryClient();
  const status = useQuery({
    queryKey: ["sync"],
    queryFn: fetchSyncStatus,
    // Poll while a sync runs so progress moves.
    refetchInterval: (q) => (q.state.data?.run?.status === "running" ? 2000 : false),
  });
  const sync = useMutation({
    mutationFn: startSync,
    onSettled: () => queryClient.invalidateQueries({ queryKey: ["sync"] }),
  });

  const run = status.data?.run ?? null;
  const stats = status.data?.stats;
  const running = run?.status === "running" || sync.isPending;

  // Reload the explorer once the library is saved and again when the sync ends.
  useEffect(() => {
    queryClient.invalidateQueries({ queryKey: ["games"] });
    queryClient.invalidateQueries({ queryKey: ["facets"] });
  }, [queryClient, run?.status, run?.games_seen]);

  return (
    <section className="panel rounded-xl border border-line bg-surface px-5 py-4 shadow-lg shadow-black/10">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="min-w-0">
          <h2 className="font-semibold tracking-tight">Your library</h2>
          <p className="mt-0.5 text-sm text-muted">
            {stats && (
              <span className="font-medium text-fg tabular-nums">
                {stats.games.toLocaleString()} games
              </span>
            )}
            {run && !running && ` · ${runLabel(run)} ${formatTime(run.finished_at ?? run.started_at)}`}
            {!run && status.data && " · never synced"}
          </p>
        </div>
        <button
          onClick={() => sync.mutate()}
          disabled={running}
          title="Fetches your owned games, then store details and reviews for new games and for games whose store data is older than 30 days, about 3 seconds per game."
          className="inline-flex items-center gap-2 rounded-md border border-control px-4 py-2 text-sm font-medium text-fg hover:border-accent hover:bg-raised disabled:cursor-not-allowed disabled:opacity-60"
        >
          <SyncIcon spinning={running} />
          {running ? "Syncing…" : "Sync library"}
        </button>
      </div>

      {run && (running || run.status === "error") && <Progress run={run} />}

      {sync.isError && (
        <ErrorBox className="mt-3">
          {sync.error instanceof ApiError && sync.error.status === 409
            ? "A sync is already running."
            : `Could not start the sync: ${sync.error.message}`}
        </ErrorBox>
      )}
      {status.isError && (
        <ErrorBox className="mt-3">Could not load sync status: {status.error.message}</ErrorBox>
      )}

      {stats && <Stats stats={stats} />}
    </section>
  );
}

function runLabel(run: SyncRun) {
  return run.status === "error" ? "last sync stopped" : "synced";
}

function Progress({ run }: { run: SyncRun }) {
  const pct =
    run.details_total > 0
      ? Math.round((run.details_fetched / run.details_total) * 100)
      : run.status === "running"
        ? 0
        : 100;
  return (
    <div className="mt-3">
      <div className="h-1.5 overflow-hidden rounded-full bg-raised">
        <div
          className={`h-full rounded-full transition-all ${run.status === "error" ? "bg-warn" : "bg-accent"}`}
          style={{ width: `${pct}%` }}
        />
      </div>
      <p className="mt-2 flex flex-wrap justify-between gap-x-4 text-sm tabular-nums text-muted">
        <span>{progressLabel(run)}</span>
        {run.seconds_left !== null && <span>{formatTimeLeft(run.seconds_left)}</span>}
      </p>
      {run.error && (
        <ErrorBox className="mt-3">
          {run.error}
          {/* A run that stops before reading any games is usually a private library. */}
          {run.games_seen === 0 && (
            <>
              {" "}
              <a href="#help" className="font-medium text-warn-fg underline underline-offset-2 hover:text-fg">
                How to fix this
              </a>
            </>
          )}
        </ErrorBox>
      )}
    </div>
  );
}

function progressLabel(run: SyncRun) {
  if (run.status === "running") {
    if (run.details_started_at === null) return "Reading your library…";
    if (run.details_fetched >= run.details_total) return "Fetching reviews…";
  }
  return `Store details ${run.details_fetched} of ${run.details_total}`;
}

function formatTimeLeft(seconds: number) {
  if (seconds < 60) return "less than a minute left";
  const minutes = Math.round(seconds / 60);
  if (minutes < 60) return `about ${minutes} min left`;
  const h = Math.floor(minutes / 60);
  const m = minutes % 60;
  return `about ${h} h${m ? ` ${m} min` : ""} left`;
}

/**
 * What the last sync saved, on one line. "No store page" and "Failed" only
 * stand out when they are above zero.
 */
function Stats({ stats }: { stats: SyncStatus["stats"] }) {
  const items = [
    { label: "with store details", value: stats.with_details, problem: false },
    { label: "with reviews", value: stats.with_reviews, problem: false },
    { label: "no store page", value: stats.unavailable, problem: true },
    { label: "failed", value: stats.failed, problem: true },
  ];
  return (
    <dl className="mt-3 flex flex-wrap gap-x-5 gap-y-1 border-t border-line pt-3 text-sm text-muted">
      {items.map(({ label, value, problem }) => (
        <div key={label} className="flex items-baseline gap-1.5">
          <dt>{label}</dt>
          <dd
            className={`order-first font-semibold tabular-nums ${problem && value > 0 ? "text-warn" : "text-fg"}`}
          >
            {value.toLocaleString()}
          </dd>
        </div>
      ))}
    </dl>
  );
}

function formatTime(iso: string) {
  return formatDateTime(iso);
}

function SyncIcon({ spinning }: { spinning: boolean }) {
  return (
    <svg
      viewBox="0 0 20 20"
      aria-hidden
      className={`h-4 w-4 ${spinning ? "animate-spin" : ""}`}
      fill="none"
      stroke="currentColor"
      strokeWidth="1.8"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <path d="M16.5 10a6.5 6.5 0 0 1-11.3 4.4M3.5 10a6.5 6.5 0 0 1 11.3-4.4" />
      <path d="M15 2.5v3.2h-3.2M5 17.5v-3.2h3.2" />
    </svg>
  );
}
