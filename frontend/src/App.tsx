import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { fetchMe, logout, type User } from "./api.ts";
import Explorer from "./Explorer.tsx";
import SyncPanel from "./SyncPanel.tsx";
import { ErrorBox } from "./ui.tsx";

export default function App() {
  const me = useQuery({ queryKey: ["me"], queryFn: fetchMe });

  return (
    <div className="min-h-screen bg-[radial-gradient(ellipse_80%_40%_at_50%_-10%,color-mix(in_oklab,var(--color-accent)_14%,transparent),transparent)]">
      <header className="sticky top-0 z-10 border-b border-line bg-canvas/80 backdrop-blur">
        <div className="mx-auto flex h-14 max-w-[1536px] items-center justify-between px-4">
          <h1 className="flex items-center gap-2.5 text-[15px] font-semibold tracking-tight">
            <Logo />
            Steamlib Explorer
          </h1>
          {me.data && <UserBadge user={me.data} />}
        </div>
      </header>

      <main className="mx-auto max-w-[1536px] px-4 py-8">
        {me.isPending && <p className="text-muted">Loading…</p>}
        {me.isError && (
          <ErrorBox>Could not reach the backend: {me.error.message}</ErrorBox>
        )}
        {me.data === null && <SignIn />}
        {me.data && (
          <div className="space-y-6">
            <SyncPanel />
            <Explorer />
          </div>
        )}
      </main>
    </div>
  );
}

function UserBadge({ user }: { user: User }) {
  const queryClient = useQueryClient();
  const signOut = useMutation({
    mutationFn: logout,
    onSuccess: () => {
      queryClient.removeQueries({ predicate: (q) => q.queryKey[0] !== "me" });
      queryClient.setQueryData(["me"], null);
    },
  });

  return (
    <div className="flex items-center gap-3">
      {user.avatar_url && (
        <img src={user.avatar_url} alt="" className="h-7 w-7 rounded-full ring-1 ring-line-strong" />
      )}
      <span className="text-sm text-muted">{user.persona_name || user.steam_id}</span>
      <button
        onClick={() => signOut.mutate()}
        disabled={signOut.isPending}
        className="rounded-md border border-line-strong px-3 py-1 text-sm text-muted hover:bg-raised hover:text-fg disabled:opacity-50"
      >
        Sign out
      </button>
    </div>
  );
}

function SignIn() {
  return (
    <section className="mx-auto mt-16 max-w-xl rounded-xl border border-line bg-surface p-10 text-center shadow-xl shadow-black/20">
      <Logo className="mx-auto mb-5 h-10 w-10" />
      <h2 className="text-2xl font-semibold tracking-tight">Explore your Steam library</h2>
      <p className="mt-2 text-muted">
        Sign in with your Steam account. Your profile and game details must be
        public for the library to load.
      </p>
      {/* A plain link: the backend redirects to Steam and back. */}
      <a
        href="/auth/steam/login"
        className="mt-6 inline-block rounded-md bg-accent px-5 py-2.5 font-medium text-on-accent hover:bg-accent-hover"
      >
        Sign in through Steam
      </a>
    </section>
  );
}

/** App mark: a stack of game cases. */
function Logo({ className = "h-6 w-6" }: { className?: string }) {
  return (
    <svg viewBox="0 0 24 24" aria-hidden className={className}>
      <rect x="3" y="4" width="4" height="16" rx="1.2" className="fill-accent" />
      <rect x="9" y="4" width="4" height="16" rx="1.2" className="fill-accent/60" />
      <rect x="15.2" y="4.6" width="4" height="16" rx="1.2" transform="rotate(-12 17.2 12.6)" className="fill-accent/35" />
    </svg>
  );
}
