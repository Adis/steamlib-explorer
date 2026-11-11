import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  ApiError,
  fetchMe,
  fetchSyncStatus,
  logout,
  startSync,
  type SyncRun,
  type SyncStatus,
  type User,
} from "./api.ts";

export default function App() {
  const me = useQuery({ queryKey: ["me"], queryFn: fetchMe });

  return (
    <div className="min-h-screen bg-slate-950 text-slate-100">
      <header className="border-b border-slate-800">
        <div className="mx-auto flex max-w-3xl items-center justify-between px-4 py-4">
          <h1 className="text-lg font-semibold">Steamlib Explorer</h1>
          {me.data && <UserBadge user={me.data} />}
        </div>
      </header>

      <main className="mx-auto max-w-3xl px-4 py-8">
        {me.isPending && <p className="text-slate-400">Loading…</p>}
        {me.isError && (
          <ErrorBox>Could not reach the backend: {me.error.message}</ErrorBox>
        )}
        {me.data === null && <SignIn />}
        {me.data && <LibrarySync />}
      </main>
    </div>
  );
}

function UserBadge({ user }: { user: User }) {
  const queryClient = useQueryClient();
  const signOut = useMutation({
    mutationFn: logout,
    onSuccess: () => queryClient.setQueryData(["me"], null),
  });

  return (
    <div className="flex items-center gap-3">
      {user.avatar_url && (
        <img src={user.avatar_url} alt="" className="h-8 w-8 rounded" />
      )}
      <span className="text-sm">{user.persona_name || user.steam_id}</span>
      <button
        onClick={() => signOut.mutate()}
        disabled={signOut.isPending}
        className="rounded border border-slate-700 px-3 py-1 text-sm hover:bg-slate-800 disabled:opacity-50"
      >
        Sign out
      </button>
    </div>
  );
}

function SignIn() {
  return (
    <section className="rounded-lg border border-slate-800 bg-slate-900 p-8 text-center">
      <h2 className="text-xl font-semibold">Explore your Steam library</h2>
      <p className="mt-2 text-slate-400">
        Sign in with your Steam account. Your profile and game details must be
        public for the library to load.
      </p>
      {/* A plain link: the backend redirects to Steam and back. */}
      <a
        href="/auth/steam/login"
        className="mt-6 inline-block rounded bg-sky-600 px-5 py-2.5 font-medium text-white hover:bg-sky-500"
      >
        Sign in through Steam
      </a>
    </section>
  );
}

function LibrarySync() {
  const queryClient = useQueryClient();
  const status = useQuery({
    queryKey: ["sync"],
    queryFn: fetchSyncStatus,
    // Poll while a refresh runs so progress moves.
    refetchInterval: (q) =>
      q.state.data?.run?.status === "running" ? 2000 : false,
  });
  const sync = useMutation({
    mutationFn: startSync,
    onSettled: () => queryClient.invalidateQueries({ queryKey: ["sync"] }),
  });

  const run = status.data?.run ?? null;
  const running = run?.status === "running" || sync.isPending;

  return (
    <section className="space-y-6">
      <div className="flex flex-wrap items-center justify-between gap-4">
        <div>
          <h2 className="text-xl font-semibold">Your library</h2>
          <p className="text-sm text-slate-400">
            Sync fetches your owned games, then store details and reviews for
            each one, about 3 seconds per game.
          </p>
        </div>
        <button
          onClick={() => sync.mutate()}
          disabled={running}
          className="rounded bg-sky-600 px-5 py-2.5 font-medium text-white hover:bg-sky-500 disabled:cursor-not-allowed disabled:opacity-50"
        >
          {running ? "Syncing…" : "Sync library"}
        </button>
      </div>

      {sync.isError && (
        <ErrorBox>
          {sync.error instanceof ApiError && sync.error.status === 409
            ? "A sync is already running."
            : `Could not start the sync: ${sync.error.message}`}
        </ErrorBox>
      )}
      {status.isError && (
        <ErrorBox>Could not load sync status: {status.error.message}</ErrorBox>
      )}

      {run && <RunPanel run={run} />}
      {status.data && <Stats stats={status.data.stats} />}

      <p className="text-xs text-slate-500">
        Every request to Steam and to this backend is logged in the backend
        console and in <code>backend/logs/api-calls.txt</code>.
      </p>
    </section>
  );
}

function RunPanel({ run }: { run: SyncRun }) {
  const pct =
    run.details_total > 0
      ? Math.round((run.details_fetched / run.details_total) * 100)
      : run.status === "running"
        ? 0
        : 100;
  const label = {
    running: "Syncing",
    ok: "Last sync finished",
    error: "Last sync stopped",
  }[run.status];

  return (
    <div className="rounded-lg border border-slate-800 bg-slate-900 p-5">
      <div className="flex items-baseline justify-between gap-4">
        <span className="font-medium">{label}</span>
        <span className="text-sm text-slate-400">
          started {formatTime(run.started_at)}
          {run.finished_at && `, ended ${formatTime(run.finished_at)}`}
        </span>
      </div>

      <div className="mt-4 h-2 overflow-hidden rounded bg-slate-800">
        <div
          className={`h-full transition-all ${run.status === "error" ? "bg-amber-500" : "bg-sky-500"}`}
          style={{ width: `${pct}%` }}
        />
      </div>
      <p className="mt-2 text-sm text-slate-400">
        {run.games_seen} games in library · store details{" "}
        {run.details_fetched} of {run.details_total}
        {run.status === "running" && run.details_total === 0 && " (reading library…)"}
      </p>

      {run.error && <ErrorBox className="mt-4">{run.error}</ErrorBox>}
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
        <div key={label} className="rounded-lg border border-slate-800 bg-slate-900 p-3">
          <dt className="text-xs text-slate-400">{label}</dt>
          <dd className="mt-1 text-2xl font-semibold tabular-nums">{value}</dd>
        </div>
      ))}
    </dl>
  );
}

function ErrorBox({
  children,
  className = "",
}: {
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <div
      role="alert"
      className={`rounded border border-amber-700 bg-amber-950 px-4 py-3 text-sm text-amber-200 ${className}`}
    >
      {children}
    </div>
  );
}

function formatTime(iso: string) {
  return new Date(iso).toLocaleString();
}
