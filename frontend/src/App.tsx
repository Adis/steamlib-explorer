import { useEffect, useLayoutEffect, useRef, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { fetchMe, fetchSetup, logout, type User } from "./api.ts";
import Explorer from "./Explorer.tsx";
import GameDetails, { gameIdOf } from "./GameDetails.tsx";
import Help from "./Help.tsx";
import Settings from "./Settings.tsx";
import Setup from "./Setup.tsx";
import SyncPanel from "./SyncPanel.tsx";
import { ErrorBox } from "./ui.tsx";
import { LibraryBig } from "lucide-react";
import { HelpIcon, SettingsIcon, SignOutIcon, type Icon } from "./icons.tsx";

export default function App() {
  const me = useQuery({ queryKey: ["me"], queryFn: fetchMe });
  const setup = useQuery({ queryKey: ["setup"], queryFn: fetchSetup, staleTime: Infinity });
  const page = useHashPage();
  // Until a Steam API key is set, the home page is the setup form.
  const needsKey = setup.data?.needs_api_key === true;
  const gameId = gameIdOf(page);
  const subpage = page === "help" || page === "settings" || gameId !== null;
  const home = !subpage && !needsKey;
  const mainRef = useRef<HTMLElement>(null);
  const firstPage = useRef(true);
  const prevPage = useRef(page);
  // Where the explorer was scrolled to, so coming back from a game's page
  // lands where the user left off.
  const homeScroll = useRef(0);
  // Whether the game page was opened from the explorer in this tab: then
  // "Back" is the browser's back, which keeps the filters and the page.
  const cameFromHome = useRef(false);
  // The game pages opened one from another ("More from" links), so a game
  // page can offer "Back to <previous game>".
  const gameTrail = useGameTrail(page, gameId);

  useEffect(() => {
    if (page !== "") return;
    const onScroll = () => (homeScroll.current = window.scrollY);
    window.addEventListener("scroll", onScroll, { passive: true });
    return () => window.removeEventListener("scroll", onScroll);
  }, [page]);

  // A page change happens without a reload, so tell assistive technology:
  // a new tab title, and focus moves to the top of the new page's content.
  useLayoutEffect(() => {
    const prev = prevPage.current;
    prevPage.current = page;
    document.title =
      page === "help" ? "Help · Steamlib Explorer" : page === "settings" ? "Settings · Steamlib Explorer" : "Steamlib Explorer";
    if (firstPage.current) {
      firstPage.current = false;
      return;
    }
    const prevGame = gameIdOf(prev);
    if (page === "" && prevGame !== null) {
      // Back from a game: same scroll position, focus on that game's link.
      window.scrollTo(0, homeScroll.current);
      const link = document.querySelector<HTMLElement>(`#games a[href="#game/${prevGame}"]`);
      (link ?? mainRef.current)?.focus({ preventScroll: true });
      return;
    }
    // Moving between game pages keeps how the first one was reached.
    if (gameIdOf(page) === null || gameIdOf(prev) === null) {
      cameFromHome.current = prev === "" && gameIdOf(page) !== null;
    }
    window.scrollTo(0, 0);
    mainRef.current?.focus({ preventScroll: true });
  }, [page]);

  return (
    <div className="min-h-screen bg-[radial-gradient(ellipse_80%_40%_at_50%_-10%,color-mix(in_oklab,var(--color-accent)_var(--glow),transparent),transparent)]">
      {/* First thing the Tab key reaches: jumps past the header and filters. */}
      <a
        href={me.data && !subpage ? "#games" : "#main"}
        onClick={(e) => {
          // A plain #fragment would be read as a page change by useHashPage.
          e.preventDefault();
          const target = document.getElementById(me.data && !subpage ? "games" : "main");
          target?.focus();
          target?.scrollIntoView();
        }}
        className="skip-link rounded-md bg-accent px-4 py-2 font-medium text-on-accent"
      >
        {me.data && !subpage ? "Skip to games" : "Skip to content"}
      </a>
      <header className="app-header sticky top-0 z-10 border-b border-line bg-canvas/80 backdrop-blur">
        <div className="mx-auto flex h-14 max-w-[1536px] items-center justify-between px-4">
          <h1>
            <a href="#" className="flex items-center gap-2.5">
              <Logo />
              <span className="font-pixel text-lg leading-none font-semibold tracking-wide">Steamlib Explorer</span>
            </a>
          </h1>
          <div className="flex items-center gap-3">
            <nav className="flex items-center gap-1">
              <NavLink href="#help" icon={HelpIcon} label="Help" current={page === "help"} />
              <NavLink href="#settings" icon={SettingsIcon} label="Settings" current={page === "settings"} />
            </nav>
            {me.data && <UserBadge user={me.data} />}
          </div>
        </div>
      </header>

      <main id="main" ref={mainRef} tabIndex={-1} className="mx-auto max-w-[1536px] px-4 py-8 outline-none">
        {page === "help" && <Help signedIn={!!me.data} />}
        {page === "settings" && <Settings signedIn={!!me.data} />}
        {gameId !== null && me.data && (
          <GameDetails
            key={gameId}
            appid={gameId}
            fromGame={gameTrail.at(-2) ?? null}
            onBack={() => {
              // Step back past every game page opened since the explorer.
              if (cameFromHome.current) window.history.go(-gameTrail.length);
              else window.location.hash = "";
            }}
          />
        )}
        {gameId !== null && me.data === null && <SignIn />}
        {!subpage && needsKey && <Setup />}
        {home && me.isPending && <p className="text-muted">Loading…</p>}
        {home && me.isError && (
          <ErrorBox>Could not reach the backend: {me.error.message}</ErrorBox>
        )}
        {home && me.data === null && <SignIn />}
        {home && me.data && (
          <div className="space-y-6">
            <SyncPanel />
            <Explorer />
          </div>
        )}
      </main>
      <FunctionKeyBar page={page} />
    </div>
  );
}

/**
 * The game pages visited in a row, oldest first: opening a game from another
 * adds it, going back removes the last one, any other page clears the trail.
 */
function useGameTrail(page: string, gameId: number | null) {
  const [trail, setTrail] = useState<{ page: string; ids: number[] }>({
    page,
    ids: gameId === null ? [] : [gameId],
  });
  if (trail.page === page) return trail.ids;
  const ids = trail.ids;
  const next =
    gameId === null
      ? []
      : ids.at(-1) === gameId
        ? ids
        : ids.at(-2) === gameId
          ? ids.slice(0, -1)
          : [...ids, gameId];
  // Adjusting state while rendering: React re-renders before painting.
  setTrail({ page, ids: next });
  return next;
}

/**
 * The page named in the address bar's #fragment: "help", "settings" or ""
 * (the library). "#help/<section>" is the Help page, opened at that section.
 */
function useHashPage() {
  const read = () => window.location.hash.replace(/^#/, "").replace(/^help\/.*/, "help");
  const [page, setPage] = useState(read);
  useEffect(() => {
    const onChange = () => setPage(read());
    window.addEventListener("hashchange", onChange);
    return () => window.removeEventListener("hashchange", onChange);
  }, []);
  return page;
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
      <span className="flex items-center gap-2 border-l border-line pl-3 text-sm text-fg">
        {user.avatar_url && (
          <img src={user.avatar_url} alt="" className="h-6 w-6 rounded-full ring-1 ring-line-strong" />
        )}
        {user.persona_name || user.steam_id}
      </span>
      <button
        onClick={() => signOut.mutate()}
        disabled={signOut.isPending}
        className="inline-flex h-8 items-center gap-1.5 rounded-md px-2.5 text-sm text-muted hover:bg-raised hover:text-fg disabled:opacity-50"
      >
        <SignOutIcon className="h-6 w-6" />
        Sign out
      </button>
    </div>
  );
}

function SignIn() {
  return (
    <section className="mx-auto mt-16 max-w-xl panel rounded-xl border border-line bg-surface p-10 text-center shadow-xl shadow-black/20">
      <Logo className="mx-auto mb-5 h-10 w-10" />
      <h2 className="text-2xl font-semibold tracking-tight">Explore your Steam library</h2>
      <p className="mt-2 text-muted">
        Sign in with your Steam account. Steam shares your library only when your
        game details are public, or when this app runs with your own Steam API
        key.{" "}
        <a href="#help" className="text-link hover:text-link-hover">
          How to set this up
        </a>
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

/** App mark: Lucide's "library-big" icon (ISC licence), in the accent colour. */
function Logo({ className = "h-6 w-6" }: { className?: string }) {
  return <LibraryBig aria-hidden className={`text-accent ${className}`} strokeWidth={2} />;
}

/** Header link with an icon and a label, highlighted on its own page. */
function NavLink({
  href,
  icon: Icon,
  label,
  current,
}: {
  href: string;
  icon: Icon;
  label: string;
  current: boolean;
}) {
  return (
    <a
      href={href}
      aria-current={current ? "page" : undefined}
      className="inline-flex h-8 items-center gap-1.5 rounded-md px-2.5 text-sm text-muted hover:bg-raised hover:text-fg aria-[current=page]:bg-raised aria-[current=page]:text-fg"
    >
      <Icon className="h-6 w-6" />
      {label}
    </a>
  );
}

/**
 * Function-key style bar along the bottom of the screen. Only the Retro DOS
 * theme shows it (index.css); the other themes hide it.
 */
function FunctionKeyBar({ page }: { page: string }) {
  const keys = [
    { n: 1, label: "Help", href: "#help", current: page === "help" },
    { n: 2, label: "Library", href: "#", current: page === "" },
    { n: 3, label: "Settings", href: "#settings", current: page === "settings" },
  ];
  return (
    <nav aria-label="Function keys" className="fkey-bar">
      {keys.map((k) => (
        <a key={k.n} href={k.href} aria-current={k.current ? "page" : undefined}>
          <span className="fkey-n">{k.n}</span>
          <span className="fkey-label">{k.label}</span>
        </a>
      ))}
    </nav>
  );
}
