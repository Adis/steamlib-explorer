import { useEffect, type ReactNode } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { ExternalLink, LibraryBig } from "lucide-react";
import { ApiError, fetchGame, type Game, type GamesPage } from "./api.ts";
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
export default function GameDetails({ appid, onBack }: { appid: number; onBack: () => void }) {
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

  const back = (
    <a
      href="#"
      onClick={(e) => {
        e.preventDefault();
        onBack();
      }}
      className="text-sm text-link hover:text-link-hover"
    >
      ← Back to your library
    </a>
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
    </article>
  );
}

function Overview({ game }: { game: Game }) {
  return (
    <section className="panel overflow-hidden rounded-xl border border-line bg-surface shadow-lg shadow-black/10 md:flex">
      <div className="aspect-[460/215] shrink-0 bg-raised md:w-[460px]">
        {game.header_image && <img src={game.header_image} alt="" className="h-full w-full object-cover" />}
      </div>
      <div className="flex min-w-0 flex-1 flex-col gap-3 p-5">
        <div>
          <h2 className="text-2xl font-semibold tracking-tight">
            {game.name}
            {isNewRelease(game) && <NewBadge game={game} className="ml-2 align-[0.2em] leading-none" />}
          </h2>
          <StatusNote game={game} />
        </div>
        {game.short_description && (
          <p className="text-[0.9375rem] leading-relaxed text-muted">{decodeEntities(game.short_description)}</p>
        )}
        <div className="mt-auto flex flex-wrap gap-2 pt-1">
          {/* steam:// links are handled by the Steam client when it's installed. */}
          <a
            href={`steam://nav/games/details/${game.appid}`}
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
    <div className="grid gap-6 md:grid-cols-2">
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
              <span className="text-faint"> · {reviewDetail(game)}</span>
            </>
          ) : (
            <Faint>No user reviews</Faint>
          )}
        </Fact>
        <Fact label="Metacritic">{game.metacritic_score ?? <Faint>–</Faint>}</Fact>
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

      <FactPanel title="Made by">
        <Fact label="Developer">{game.developers.join(", ") || <Faint>–</Faint>}</Fact>
        <Fact label="Publisher">{game.publishers.join(", ") || <Faint>–</Faint>}</Fact>
      </FactPanel>

      <FactPanel title="Store tags">
        <Fact label="Genres">
          <Tags values={game.genres} />
        </Fact>
        <Fact label="Features">
          <Tags values={game.categories} />
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

function Tags({ values }: { values: string[] }) {
  if (values.length === 0) return <Faint>–</Faint>;
  return (
    <ul className="flex flex-wrap gap-1">
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
