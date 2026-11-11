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
    <section className="rounded-lg border border-slate-800 bg-slate-900 p-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="text-sm">
          <span className="font-semibold">Your library</span>
          <span className="text-slate-400">
            {stats && ` · ${stats.games.toLocaleString()} games`}
            {run && !running && ` · ${runLabel(run)} ${formatTime(run.finished_at ?? run.started_at)}`}
            {!run && status.data && " · never synced"}
          </span>
          {status.data && (
            <button
              onClick={() => setShowDetails(!showDetails)}
              aria-expanded={showDetails}
              className="ml-3 text-sky-400 hover:text-sky-300"
            >
              {showDetails ? "Hide details" : "Details"}
            </button>
          )}
        </div>
        <button
          onClick={() => sync.mutate()}
          disabled={running}
          title="Fetches your owned games, then store details and reviews for new games, about 3 seconds per game"
          className="rounded bg-sky-600 px-4 py-2 text-sm font-medium text-white hover:bg-sky-500 disabled:cursor-not-allowed disabled:opacity-50"
        >
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
          <p className="text-xs text-slate-500">
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
      <div className="h-2 overflow-hidden rounded bg-slate-800">
        <div
          className={`h-full transition-all ${run.status === "error" ? "bg-amber-500" : "bg-sky-500"}`}
          style={{ width: `${pct}%` }}
        />
      </div>
      <p className="mt-2 text-sm text-slate-400">
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
        <div key={label} className="rounded-lg border border-slate-800 bg-slate-950 p-3">
          <dt className="text-xs text-slate-400">{label}</dt>
          <dd className="mt-1 text-2xl font-semibold tabular-nums">{value}</dd>
        </div>
      ))}
    </dl>
  );
}

function formatTime(iso: string) {
  return new Date(iso).toLocaleString();
}
