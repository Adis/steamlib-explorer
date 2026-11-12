import type { Game } from "./api.ts";
import { formatHours, reviewColor } from "./GamesTable.tsx";

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
      className="group flex h-full flex-col overflow-hidden rounded-xl border border-line bg-surface shadow-lg shadow-black/10 transition duration-200 hover:-translate-y-0.5 hover:border-line-strong hover:shadow-black/30"
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
        {game.review_pct !== null && (
          <span
            title={`${game.review_score_desc ?? ""} · ${game.reviews_total?.toLocaleString() ?? 0} reviews`}
            className={`absolute right-2 top-2 rounded-md bg-canvas/85 px-1.5 py-0.5 text-xs font-semibold tabular-nums backdrop-blur ${reviewColor(game.review_pct)}`}
          >
            {Math.round(game.review_pct)}%
          </span>
        )}
        {game.playtime_2weeks_minutes > 0 && (
          <span className="absolute left-2 top-2 rounded-md bg-accent px-1.5 py-0.5 text-[11px] font-semibold text-on-accent">
            Recently played
          </span>
        )}
      </div>

      <div className="flex flex-1 flex-col gap-1 p-3">
        <h3 className="truncate font-medium tracking-tight group-hover:text-link" title={game.name}>
          {game.name}
        </h3>
        <p className="truncate text-xs text-faint">{subtitle(game)}</p>

        <div className="mt-auto flex items-center justify-between gap-2 pt-2 text-xs">
          <span className={played ? "tabular-nums text-fg" : "text-faint"}>
            {played ? formatHours(game.playtime_minutes) : "Never played"}
            {game.last_played_at && (
              <span className="text-faint">
                {" · "}
                {new Date(game.last_played_at).toLocaleDateString()}
              </span>
            )}
          </span>
          <span className="flex shrink-0 items-center gap-2 text-faint">
            {game.metacritic_score !== null && (
              <span title="Metacritic" className="rounded border border-line-strong px-1 tabular-nums">
                {game.metacritic_score}
              </span>
            )}
            {game.release_date && <span className="tabular-nums">{game.release_date.slice(0, 4)}</span>}
          </span>
        </div>
      </div>
    </a>
  );
}

function subtitle(game: Game) {
  if (game.details_status === "pending") return "Store details not fetched yet";
  if (game.details_status === "unavailable") return "No longer on the Steam store";
  return game.genres.join(", ") || " ";
}
