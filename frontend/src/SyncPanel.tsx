import { useEffect, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ApiError, fetchSyncStatus, startSync, type SyncRun, type SyncStatus } from "./api.ts";
import { ErrorBox } from "./ui.tsx";

/** Library status, the Sync button, and progress while a sync runs. */
export default function SyncPanel() {
  const queryClient = useQueryClient();
  const [showDetails, setShowDetails] = useState(false);
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
    <section className="rounded-xl border border-line bg-surface px-5 py-4 shadow-lg shadow-black/10">
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
            {status.data && (
              <button
                onClick={() => setShowDetails(!showDetails)}
                aria-expanded={showDetails}
                className="ml-3 text-link hover:text-link-hover"
              >
                {showDetails ? "Hide details" : "Details"}
              </button>
            )}
          </p>
        </div>
        <button
          onClick={() => sync.mutate()}
          disabled={running}
          title="Fetches your owned games, then store details and reviews for new games, about 3 seconds per game"
          className="inline-flex items-center gap-2 rounded-md bg-accent px-4 py-2 text-sm font-medium text-on-accent shadow-sm hover:bg-accent-hover disabled:cursor-not-allowed disabled:opacity-60"
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

      {showDetails && stats && (
        <div className="mt-4 space-y-3">
          <Stats stats={stats} />
          <p className="text-xs text-faint">
            A sync fetches store details and reviews for new games and for games
            whose store data is older than 30 days, about 3 seconds per game.
            Every request is logged in the backend console and in{" "}
            <code>backend/logs/api-calls.txt</code>.
          </p>
        </div>
      )}
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
          className={`h-full rounded-full transition-all ${run.status === "error" ? "bg-amber-500" : "bg-accent"}`}
          style={{ width: `${pct}%` }}
        />
      </div>
      <p className="mt-2 text-sm tabular-nums text-muted">
        {run.status === "running" && run.details_total === 0
          ? "Reading your library…"
          : `Store details ${run.details_fetched} of ${run.details_total}`}
      </p>
      {run.error && <ErrorBox className="mt-3">{run.error}</ErrorBox>}
    </div>
  );
}

function Stats({ stats }: { stats: SyncStatus["stats"] }) {
  const items = [
    ["Games owned", stats.games],
    ["With store details", stats.with_details],
    ["With reviews", stats.with_reviews],
    ["No store page", stats.unavailable],
    ["Failed", stats.failed],
  ] as const;
  return (
    <dl className="grid grid-cols-2 gap-3 sm:grid-cols-5">
      {items.map(([label, value]) => (
        <div key={label} className="rounded-lg border border-line bg-canvas/60 p-3">
          <dt className="text-xs text-muted">{label}</dt>
          <dd className="mt-1 text-2xl font-semibold tracking-tight tabular-nums">{value}</dd>
        </div>
      ))}
    </dl>
  );
}

function formatTime(iso: string) {
  return new Date(iso).toLocaleString();
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
