import type { Game } from "./api.ts";
import { formatHours, reviewColor, reviewDetail } from "./GamesTable.tsx";
import { formatDate } from "./ui.tsx";

/** The filtered games as cover-art cards. */
export default function GamesGrid({ games }: { games: Game[] }) {
  return (
    <ul className="grid grid-cols-[repeat(auto-fill,minmax(220px,1fr))] gap-4">
      {games.map((g) => (
        <li key={g.appid}>
          <GameCard game={g} />
        </li>
      ))}
    </ul>
  );
}

function GameCard({ game }: { game: Game }) {
  const played = game.playtime_minutes > 0;
  return (
    <a
      href={game.store_url}
      target="_blank"
      rel="noreferrer"
      className="group flex h-full flex-col overflow-hidden panel rounded-xl border border-line bg-surface shadow-lg shadow-black/10 transition duration-200 hover:-translate-y-0.5 hover:border-line-strong hover:shadow-black/30"
    >
      <div className="relative aspect-[460/215] overflow-hidden bg-raised">
        {game.header_image && (
          <img
            src={game.header_image}
            alt=""
            loading="lazy"
            className="h-full w-full object-cover transition duration-300 group-hover:scale-[1.03]"
          />
        )}
        {game.playtime_2weeks_minutes > 0 && (
          <span className="absolute left-2 top-2 rounded-md bg-accent px-1.5 py-0.5 text-[0.6875rem] font-semibold text-on-accent">
            Recently played
          </span>
        )}
      </div>

      <div className="flex flex-1 flex-col p-3">
        <h3 className="truncate font-medium tracking-tight group-hover:text-link" title={game.name}>
          {game.name}
        </h3>
        <Rating game={game} />

        <div className="mt-2 flex items-center justify-between gap-2 text-xs">
          <span className={played ? "tabular-nums text-fg" : "text-faint"}>
            {played ? formatHours(game.playtime_minutes) : "Never played"}
            {game.last_played_at && (
              <span className="text-faint">
                {" · "}
                {formatDate(game.last_played_at)}
              </span>
            )}
          </span>
          {game.release_date && (
            <span className="shrink-0 tabular-nums text-faint">{game.release_date.slice(0, 4)}</span>
          )}
        </div>

        <div className="mt-auto pt-3">
          <Genres game={game} />
        </div>
      </div>
    </a>
  );
}

/** Steam's rating in words, e.g. "Very Positive · 80% of 68,133". */
function Rating({ game }: { game: Game }) {
  if (game.details_status === "pending")
    return <p className="mt-0.5 truncate text-xs text-faint">Store details not fetched yet</p>;
  if (game.details_status === "unavailable")
    return <p className="mt-0.5 truncate text-xs text-faint">No longer on the Steam store</p>;
  if (!game.review_score_desc || game.review_pct === null)
    return <p className="mt-0.5 truncate text-xs text-faint">No user reviews</p>;
  return (
    <p className="mt-0.5 truncate text-xs">
      <span className={`font-medium ${reviewColor(game.review_pct)}`}>{game.review_score_desc}</span>
      <span className="text-faint"> · {reviewDetail(game)}</span>
    </p>
  );
}

const MAX_GENRES = 2;

/** Up to two genres as small tags along the bottom of the card, then "+N". */
function Genres({ game }: { game: Game }) {
  if (game.genres.length === 0) return null;
  const shown = game.genres.slice(0, MAX_GENRES);
  const more = game.genres.length - shown.length;
  const tag = "min-w-0 truncate rounded-full bg-raised px-2 py-0.5 text-[0.6875rem] text-muted";
  return (
    <ul className="flex gap-1 border-t border-line pt-2.5" title={game.genres.join(", ")}>
      {shown.map((g) => (
        <li key={g} className={tag}>
          {g}
        </li>
      ))}
      {more > 0 && <li className={`${tag} shrink-0`}>+{more}</li>}
    </ul>
  );
}
