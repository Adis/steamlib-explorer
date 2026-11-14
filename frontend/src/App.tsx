import { useEffect, useRef, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { fetchMe, logout, type User } from "./api.ts";
import Explorer from "./Explorer.tsx";
import Help from "./Help.tsx";
import Settings from "./Settings.tsx";
import SyncPanel from "./SyncPanel.tsx";
import { ErrorBox } from "./ui.tsx";
import { CircleHelp, LibraryBig, LogOut, Settings as SettingsIcon, type LucideIcon } from "lucide-react";

export default function App() {
  const me = useQuery({ queryKey: ["me"], queryFn: fetchMe });
  const page = useHashPage();
  const subpage = page === "help" || page === "settings";
  const mainRef = useRef<HTMLElement>(null);
  const firstPage = useRef(true);

  // A page change happens without a reload, so tell assistive technology:
  // a new tab title, and focus moves to the top of the new page's content.
  useEffect(() => {
    document.title =
      page === "help" ? "Help · Steamlib Explorer" : page === "settings" ? "Settings · Steamlib Explorer" : "Steamlib Explorer";
    if (firstPage.current) {
      firstPage.current = false;
      return;
    }
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
              <NavLink href="#help" icon={CircleHelp} label="Help" current={page === "help"} />
              <NavLink href="#settings" icon={SettingsIcon} label="Settings" current={page === "settings"} />
            </nav>
            {me.data && <UserBadge user={me.data} />}
          </div>
        </div>
      </header>

      <main id="main" ref={mainRef} tabIndex={-1} className="mx-auto max-w-[1536px] px-4 py-8 outline-none">
        {page === "help" && <Help signedIn={!!me.data} />}
        {page === "settings" && <Settings signedIn={!!me.data} />}
        {!subpage && me.isPending && <p className="text-muted">Loading…</p>}
        {!subpage && me.isError && (
          <ErrorBox>Could not reach the backend: {me.error.message}</ErrorBox>
        )}
        {!subpage && me.data === null && <SignIn />}
        {!subpage && me.data && (
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

/** The page named in the address bar's #fragment: "help", "settings" or "" (the library). */
function useHashPage() {
  const read = () => window.location.hash.replace(/^#/, "");
  const [page, setPage] = useState(read);
  useEffect(() => {
    const onChange = () => {
      setPage(read());
      window.scrollTo(0, 0);
    };
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
        <LogOut aria-hidden className="h-4 w-4" />
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
  icon: LucideIcon;
  label: string;
  current: boolean;
}) {
  return (
    <a
      href={href}
      aria-current={current ? "page" : undefined}
      className="inline-flex h-8 items-center gap-1.5 rounded-md px-2.5 text-sm text-muted hover:bg-raised hover:text-fg aria-[current=page]:bg-raised aria-[current=page]:text-fg"
    >
      <Icon aria-hidden className="h-4 w-4" />
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
    { n: 2, label: "Library", href: "#", current: page !== "help" && page !== "settings" },
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
