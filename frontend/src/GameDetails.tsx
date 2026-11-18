import { useEffect, type ReactNode } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Check, ExternalLink, LibraryBig } from "lucide-react";
import { ApiError, fetchDeveloperGames, fetchGame, type DeveloperGame, type Game, type GamesPage } from "./api.ts";
import { formatHours, ReviewLabel, reviewDetail } from "./GamesTable.tsx";
import { ErrorBox, formatDate, isNewRelease, NewBadge } from "./ui.tsx";

/** The address of a game's details page; the explorer's filters stay in the query string. */
export const gameHref = (appid: number) => `#game/${appid}`;

/** The appid in a "game/620" page name, or null for any other page. */
export function gameIdOf(page: string): number | null {
  const m = /^game\/(\d+)$/.exec(page);
  return m ? Number(m[1]) : null;
}

/**
 * One game from the library: what the app knows about it, and buttons to
 * open it in the Steam client or on the store.
 */
export default function GameDetails({
  appid,
  fromGame,
  onBack,
}: {
  appid: number;
  /** The game page this one was opened from, if any: "Back" returns there first. */
  fromGame: number | null;
  onBack: () => void;
}) {
  const queryClient = useQueryClient();
  const game = useQuery({
    queryKey: ["game", appid],
    queryFn: () => fetchGame(appid),
    // Coming from the explorer, the game is already in a loaded page of
    // results: show that at once while the fresh copy loads.
    placeholderData: () => {
      for (const [, page] of queryClient.getQueriesData<GamesPage>({ queryKey: ["games"] })) {
        const g = page?.games.find((x) => x.appid === appid);
        if (g) return g;
      }
      return undefined;
    },
  });

  useEffect(() => {
    if (game.data) document.title = `${game.data.name} · Steamlib Explorer`;
  }, [game.data]);

  const fromName = fromGame !== null ? queryClient.getQueryData<Game>(["game", fromGame])?.name : undefined;
  // A block, not inline: the article's space-y margin doesn't apply to inline links.
  const back = (
    <nav aria-label="Back" className="flex flex-wrap items-baseline gap-x-3 gap-y-1 text-sm">
      {fromGame !== null && (
        <>
          <a
            href={gameHref(fromGame)}
            onClick={(e) => {
              e.preventDefault();
              window.history.back();
            }}
            className="text-link hover:text-link-hover"
          >
            ← Back to {fromName ?? "the previous game"}
          </a>
          <span aria-hidden className="text-faint">·</span>
        </>
      )}
      <a
        href="#"
        onClick={(e) => {
          e.preventDefault();
          onBack();
        }}
        className="text-link hover:text-link-hover"
      >
        {fromGame !== null ? "Your library" : "← Back to your library"}
      </a>
    </nav>
  );

  if (game.isPending) return <article className="mx-auto max-w-4xl">{back}<p className="mt-4 text-muted">Loading…</p></article>;
  if (game.isError) {
    const notOwned = game.error instanceof ApiError && game.error.status === 404;
    return (
      <article className="mx-auto max-w-4xl space-y-4">
        {back}
        <ErrorBox>
          {notOwned ? "This game isn't in your library." : `Could not load this game: ${game.error.message}`}
        </ErrorBox>
      </article>
    );
  }
  return (
    <article className="mx-auto max-w-4xl space-y-6">
      {back}
      <Overview game={game.data} />
      <Facts game={game.data} />
      {game.data.developers.length > 0 && (
        <MoreFromDeveloper developer={game.data.developers[0]} appid={game.data.appid} />
      )}
    </article>
  );
}

function Overview({ game }: { game: Game }) {
  return (
    <section className="panel overflow-hidden rounded-xl border border-line bg-surface shadow-lg shadow-black/10 lg:flex lg:items-start">
      {/* Steam's header art at its own 460×215 shape, never cropped: the text
          beside it may be taller, so the art stays at the top. */}
      <div className="aspect-[460/215] shrink-0 bg-raised lg:w-[460px]">
        {game.header_image && <img src={game.header_image} alt="" className="h-full w-full object-cover" />}
      </div>
      <div className="flex min-w-0 flex-1 flex-col gap-3 p-5">
        <div>
          <h2 className="text-2xl font-semibold tracking-tight">
            {game.name}
            {isNewRelease(game) && <NewBadge game={game} className="ml-2 align-[0.2em] leading-none" />}
          </h2>
          <Credits game={game} />
          <StatusNote game={game} />
        </div>
        <Tags values={game.genres} label="Genres" />
        {game.short_description && (
          <p className="text-[0.9375rem] leading-relaxed text-muted">{decodeEntities(game.short_description)}</p>
        )}
        <div className="mt-auto flex flex-wrap gap-2 pt-1">
          {/* steam:// links are handled by the Steam client when it's installed. */}
          <a
            href={`steam://nav/games/details/${game.appid}`}
            title="Opens this game in the Steam app. Needs Steam installed on this computer."
            className="inline-flex items-center gap-2 rounded-md bg-accent px-4 py-2 text-sm font-medium text-on-accent hover:bg-accent-hover"
          >
            <LibraryBig aria-hidden className="h-4 w-4" />
            Open in library
          </a>
          <a
            href={game.store_url}
            target="_blank"
            rel="noreferrer"
            className="inline-flex items-center gap-2 rounded-md border border-line-strong px-4 py-2 text-sm font-medium text-fg hover:bg-raised"
          >
            <ExternalLink aria-hidden className="h-4 w-4" />
            Open store page
            <span className="sr-only"> (opens in a new tab)</span>
          </a>
        </div>
      </div>
    </section>
  );
}

function StatusNote({ game }: { game: Game }) {
  const note =
    game.details_status === "pending"
      ? "Store details not fetched yet: sync your library to get them."
      : game.details_status === "unavailable"
        ? "No longer on the Steam store."
        : game.details_status === "error"
          ? "Fetching store details failed during the last sync."
          : null;
  return note ? <p className="mt-1 text-sm text-faint">{note}</p> : null;
}

function Facts({ game }: { game: Game }) {
  const played = game.playtime_minutes > 0;
  return (
    <div className="grid items-start gap-6 md:grid-cols-2">
      <FactPanel title="Your play">
        <Fact label="Playtime">{played ? formatHours(game.playtime_minutes) : <Faint>Never played</Faint>}</Fact>
        <Fact label="Last 2 weeks">
          {game.playtime_2weeks_minutes > 0 ? formatHours(game.playtime_2weeks_minutes) : <Faint>–</Faint>}
        </Fact>
        <Fact label="Last played">{game.last_played_at ? formatDate(game.last_played_at) : <Faint>–</Faint>}</Fact>
      </FactPanel>

      <FactPanel title="On Steam">
        <Fact label="Reviews">
          {game.review_pct !== null && game.review_score_desc ? (
            <>
              <ReviewLabel pct={game.review_pct}>{game.review_score_desc}</ReviewLabel>
              <span className="text-faint">
                {" · "}
                <span className="whitespace-nowrap">{reviewDetail(game)}</span>
              </span>
            </>
          ) : (
            <Faint>No user reviews</Faint>
          )}
        </Fact>
        <Fact label="Metacritic">
          <Metacritic game={game} />
        </Fact>
        <Fact label="Released">
          {game.coming_soon ? (
            <>Coming soon{game.release_date_text && <Faint> · {game.release_date_text}</Faint>}</>
          ) : game.release_date ? (
            formatDate(game.release_date + "T00:00:00")
          ) : (
            game.release_date_text || <Faint>–</Faint>
          )}
        </Fact>
        <Fact label="Price">{formatPrice(game) ?? <Faint>–</Faint>}</Fact>
        <Fact label="Platforms">
          {[game.platforms.windows && "Windows", game.platforms.mac && "macOS", game.platforms.linux && "Linux"]
            .filter(Boolean)
            .join(", ") || <Faint>–</Faint>}
        </Fact>
      </FactPanel>

    </div>
  );
}

function FactPanel({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="panel rounded-xl border border-line bg-surface px-5 py-4">
      <h3 className="mb-2 text-xs font-semibold uppercase tracking-wider text-faint">{title}</h3>
      <dl className="divide-y divide-line text-sm">{children}</dl>
    </section>
  );
}

function Fact({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="flex gap-4 py-2">
      <dt className="w-28 shrink-0 text-muted">{label}</dt>
      <dd className="min-w-0 flex-1 tabular-nums">{children}</dd>
    </div>
  );
}

/** "By Developer · Published by Publisher", or just "By Studio" when they're the same. */
function Credits({ game }: { game: Game }) {
  const dev = game.developers.join(", ");
  const pub = game.publishers.join(", ");
  if (!dev && !pub) return null;
  return (
    <p className="mt-1 text-sm text-muted">
      {dev && <>By <span className="text-fg">{dev}</span></>}
      {pub && pub !== dev && (
        <>
          {dev && " · "}Published by <span className="text-fg">{pub}</span>
        </>
      )}
    </p>
  );
}

/** The score, linked to its Metacritic page when Steam gives one. */
function Metacritic({ game }: { game: Game }) {
  if (game.metacritic_score === null) return <Faint>–</Faint>;
  if (!game.metacritic_url) return <>{game.metacritic_score}</>;
  return (
    <a
      href={game.metacritic_url}
      target="_blank"
      rel="noreferrer"
      className="inline-flex items-center gap-1 text-link underline hover:text-link-hover"
    >
      {game.metacritic_score}
      <ExternalLink aria-hidden className="h-3.5 w-3.5" />
      <span className="sr-only"> on Metacritic (opens in a new tab)</span>
    </a>
  );
}

function Tags({ values, label }: { values: string[]; label: string }) {
  if (values.length === 0) return null;
  return (
    <ul aria-label={label} className="flex flex-wrap gap-1">
      {values.map((v) => (
        <li key={v} className="rounded-full bg-raised px-2 py-0.5 text-xs text-muted">
          {v}
        </li>
      ))}
    </ul>
  );
}

function Faint({ children }: { children: ReactNode }) {
  return <span className="text-faint">{children}</span>;
}

/** "Free", or the price in its currency as the store last showed it. */
function formatPrice(game: Game) {
  if (game.is_free) return "Free";
  if (game.price_cents === null || !game.price_currency) return null;
  try {
    return new Intl.NumberFormat(undefined, { style: "currency", currency: game.price_currency }).format(
      game.price_cents / 100,
    );
  } catch {
    return `${(game.price_cents / 100).toFixed(2)} ${game.price_currency}`;
  }
}

/** Steam's short descriptions can contain HTML entities such as &amp;quot;. */
function decodeEntities(s: string) {
  if (!s.includes("&")) return s;
  const doc = new DOMParser().parseFromString(s, "text/html");
  return doc.documentElement.textContent ?? s;
}

/**
 * The developer's other games on the Steam store. Games in the library are
 * marked and open their own details page; the rest open the store.
 */
function MoreFromDeveloper({ developer, appid }: { developer: string; appid: number }) {
  const more = useQuery({
    queryKey: ["developer-games", developer],
    queryFn: () => fetchDeveloperGames(developer),
    staleTime: 24 * 60 * 60 * 1000, // the backend caches the store's answer for a day too
    retry: false,
  });
  // Games in the library first, each group in the store's order.
  const games = (more.data?.games.filter((g) => g.appid !== appid) ?? []).sort(
    (a, b) => Number(b.owned) - Number(a.owned),
  );
  const owned = games.filter((g) => g.owned).length;

  // Nothing else from this developer: no section at all.
  if (more.data && games.length === 0) return null;

  return (
    <section aria-labelledby="more-heading" className="space-y-3">
      <div className="flex flex-wrap items-baseline justify-between gap-2">
        <h3 id="more-heading" className="text-lg font-semibold tracking-tight">
          More from {developer}
        </h3>
        {more.data && games.length > 0 && (
          <p className="text-sm text-muted tabular-nums">
            {games.length} {games.length === 1 ? "game" : "games"} · {owned} in your library
          </p>
        )}
      </div>
      {more.isPending && <p className="text-sm text-muted">Asking the Steam store…</p>}
      {more.isError && <p className="text-sm text-faint">Could not load them: {more.error.message}</p>}
      {games.length > 0 && (
        <ul className="grid grid-cols-[repeat(auto-fill,minmax(200px,1fr))] gap-3">
          {games.map((g) => (
            <li key={g.appid}>
              <DeveloperGameCard game={g} />
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}

function DeveloperGameCard({ game }: { game: DeveloperGame }) {
  // Owned games open their page here; the others open the store.
  const link = game.owned
    ? { href: gameHref(game.appid) }
    : { href: game.store_url, target: "_blank", rel: "noreferrer" };
  return (
    <a
      {...link}
      className={`group flex h-full flex-col overflow-hidden panel rounded-lg border bg-surface transition hover:border-line-strong ${
        game.owned ? "owned-card border-accent" : "border-line"
      }`}
    >
      <div className="relative aspect-[231/87] bg-raised">
        {game.capsule_image && (
          <img
            src={game.capsule_image}
            alt=""
            loading="lazy"
            onError={(e) => (e.currentTarget.style.visibility = "hidden")}
            className="h-full w-full object-cover"
          />
        )}
        {game.owned && (
          <span className="in-library-badge absolute left-1.5 top-1.5 inline-flex items-center gap-1 rounded-md border border-accent bg-surface px-1.5 py-0.5 text-[0.6875rem] font-semibold text-fg shadow-sm shadow-black/30">
            <Check aria-hidden className="h-3 w-3 text-accent" strokeWidth={3} />
            In library
          </span>
        )}
      </div>
      <div className="flex flex-1 flex-col gap-0.5 p-2.5">
        <span className="truncate text-sm font-medium tracking-tight group-hover:text-link" title={game.name}>
          {game.name}
          {!game.owned && <span className="sr-only"> (not in your library, opens the store in a new tab)</span>}
        </span>
        <span className="truncate text-xs">
          {game.review_pct !== null && game.review_score_desc ? (
            <ReviewLabel pct={game.review_pct}>{game.review_score_desc}</ReviewLabel>
          ) : (
            <Faint>No user reviews</Faint>
          )}
        </span>
        <span className="mt-auto flex items-center justify-between gap-2 pt-1 text-xs text-faint">
          <span className="truncate">{game.released_text}</span>
          {!game.owned && <ExternalLink aria-hidden className="h-3 w-3 shrink-0" />}
        </span>
      </div>
    </a>
  );
}
