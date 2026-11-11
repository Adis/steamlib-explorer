import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { fetchMe, logout, type User } from "./api.ts";
import Explorer from "./Explorer.tsx";
import SyncPanel from "./SyncPanel.tsx";
import { ErrorBox } from "./ui.tsx";

export default function App() {
  const me = useQuery({ queryKey: ["me"], queryFn: fetchMe });

  return (
    <div className="min-h-screen bg-slate-950 text-slate-100">
      <header className="border-b border-slate-800">
        <div className="mx-auto flex max-w-[1536px] items-center justify-between px-4 py-4">
          <h1 className="text-lg font-semibold">Steamlib Explorer</h1>
          {me.data && <UserBadge user={me.data} />}
        </div>
      </header>

      <main className="mx-auto max-w-[1536px] px-4 py-8">
        {me.isPending && <p className="text-slate-400">Loading…</p>}
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
    <section className="mx-auto max-w-xl rounded-lg border border-slate-800 bg-slate-900 p-8 text-center">
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
