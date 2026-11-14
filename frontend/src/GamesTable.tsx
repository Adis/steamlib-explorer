import {
  createColumnHelper,
  flexRender,
  getCoreRowModel,
  useReactTable,
} from "@tanstack/react-table";
import type { Game } from "./api.ts";
import { useTableColumns } from "./tableColumns.ts";
import { formatDate } from "./ui.tsx";

/** Sort keys accepted by GET /api/games, by column id. */
const SORT_KEYS: Record<string, string> = {
  name: "name",
  playtime: "playtime",
  recent: "recent",
  last_played: "last_played",
  review: "review",
  metacritic: "metacritic",
  release: "release",
};

const col = createColumnHelper<Game>();

const columns = [
  col.accessor("name", {
    id: "name",
    header: "Game",
    cell: ({ row }) => <NameCell game={row.original} />,
  }),
  col.accessor("playtime_minutes", {
    id: "playtime",
    header: "Playtime",
    cell: ({ getValue }) =>
      getValue() > 0 ? formatHours(getValue()) : <Muted>never played</Muted>,
    meta: { numeric: true },
  }),
  col.accessor("playtime_2weeks_minutes", {
    id: "recent",
    header: "2 weeks",
    cell: ({ getValue }) => (getValue() > 0 ? formatHours(getValue()) : <Muted>–</Muted>),
    meta: { numeric: true },
  }),
  col.accessor("last_played_at", {
    id: "last_played",
    header: "Last played",
    cell: ({ getValue }) => {
      const v = getValue();
      return v ? formatDate(v) : <Muted>–</Muted>;
    },
    meta: { numeric: true },
  }),
  col.accessor("review_pct", {
    id: "review",
    header: "Reviews",
    cell: ({ row }) => <ReviewCell game={row.original} />,
    meta: { numeric: true },
  }),
  col.accessor("metacritic_score", {
    id: "metacritic",
    header: "Metacritic",
    cell: ({ getValue }) => getValue() ?? <Muted>–</Muted>,
    meta: { numeric: true },
  }),
  col.accessor("release_date", {
    id: "release",
    header: "Released",
    cell: ({ row }) => {
      const g = row.original;
      if (g.release_date) return g.release_date.slice(0, 4);
      return <Muted>{g.release_date_text || "–"}</Muted>;
    },
    meta: { numeric: true },
  }),
  col.accessor("platforms", {
    id: "platforms",
    header: "Platforms",
    cell: ({ getValue }) => <Platforms p={getValue()} />,
  }),
];

type Props = {
  games: Game[];
  sort: string;
  descending: boolean;
  onSort: (sort: string, order: "asc" | "desc" | null) => void;
};

export default function GamesTable({ games, sort, descending, onSort }: Props) {
  const { hidden } = useTableColumns();
  // Sorting and paging happen in the database; the table only renders a page.
  const table = useReactTable({
    data: games,
    columns,
    getCoreRowModel: getCoreRowModel(),
    getRowId: (g) => String(g.appid),
    manualSorting: true,
    manualPagination: true,
    state: {
      sorting: [{ id: sort, desc: descending }],
      columnVisibility: Object.fromEntries(hidden.map((id) => [id, false])),
    },
  });

  const clickHeader = (id: string) => {
    const key = SORT_KEYS[id];
    if (!key) return;
    if (key === sort) onSort(key, descending ? "asc" : "desc");
    else onSort(key, null); // each column starts in its natural direction
  };

  return (
    <div className="overflow-x-auto panel rounded-xl border border-line bg-surface/40 shadow-lg shadow-black/10">
      <table className="w-full text-sm">
        <thead className="border-b border-line bg-surface text-left text-[11px] uppercase tracking-wider text-faint">
          {table.getHeaderGroups().map((hg) => (
            <tr key={hg.id}>
              {hg.headers.map((h) => {
                const sortable = h.column.id in SORT_KEYS;
                const sorted = h.column.getIsSorted();
                const numeric = isNumeric(h.column.columnDef.meta);
                return (
                  <th
                    key={h.id}
                    scope="col"
                    aria-sort={sorted ? (sorted === "desc" ? "descending" : "ascending") : undefined}
                    className={`whitespace-nowrap px-3 py-2.5 font-semibold ${numeric ? "text-right" : ""}`}
                  >
                    {sortable ? (
                      <button
                        onClick={() => clickHeader(h.column.id)}
                        className={`inline-flex items-center gap-1 uppercase hover:text-fg ${sorted ? "text-fg" : ""}`}
                      >
                        {flexRender(h.column.columnDef.header, h.getContext())}
                        <span aria-hidden className="w-3">
                          {sorted === "asc" ? "▲" : sorted === "desc" ? "▼" : ""}
                        </span>
                      </button>
                    ) : (
                      flexRender(h.column.columnDef.header, h.getContext())
                    )}
                  </th>
                );
              })}
            </tr>
          ))}
        </thead>
        <tbody className="divide-y divide-line/70">
          {table.getRowModel().rows.map((row) => (
            <tr key={row.id} className="transition-colors hover:bg-raised/50">
              {row.getVisibleCells().map((cell) => (
                <td
                  key={cell.id}
                  className={`px-3 py-2 align-middle ${isNumeric(cell.column.columnDef.meta) ? "whitespace-nowrap text-right tabular-nums" : ""}`}
                >
                  {flexRender(cell.column.columnDef.cell, cell.getContext())}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function isNumeric(meta: unknown) {
  return (meta as { numeric?: boolean } | undefined)?.numeric === true;
}

function NameCell({ game }: { game: Game }) {
  return (
    <div className="flex min-w-56 items-center gap-3">
      {game.header_image ? (
        <img
          src={game.header_image}
          alt=""
          loading="lazy"
          className="h-[37px] w-[80px] shrink-0 rounded object-cover ring-1 ring-white/5"
        />
      ) : (
        <div className="h-[37px] w-[80px] shrink-0 rounded bg-raised" />
      )}
      <div className="min-w-0">
        <a
          href={game.store_url}
          target="_blank"
          rel="noreferrer"
          className="font-medium tracking-tight hover:text-link"
        >
          {game.name}
        </a>
        <div className="truncate text-xs text-faint">
          {game.details_status === "pending"
            ? "Store details not fetched yet"
            : game.details_status === "unavailable"
              ? "No longer on the Steam store"
              : game.genres.join(", ")}
        </div>
      </div>
    </div>
  );
}

/** Steam's rating in words, with the share and count beneath, as on the cards. */
function ReviewCell({ game }: { game: Game }) {
  if (game.review_pct === null) return <Muted>–</Muted>;
  return (
    <span>
      <span className={`font-medium ${reviewColor(game.review_pct)}`}>
        {game.review_score_desc || `${Math.round(game.review_pct)}%`}
      </span>
      <span className="block text-xs text-faint">{reviewDetail(game)}</span>
    </span>
  );
}

/** "80% of 68,133": the positive share and the number of reviews. */
export function reviewDetail(game: Game) {
  const pct = game.review_pct === null ? "" : `${Math.round(game.review_pct)}%`;
  return game.reviews_total ? `${pct} of ${game.reviews_total.toLocaleString()}` : pct;
}

function Platforms({ p }: { p: Game["platforms"] }) {
  const supported = [
    ["Win", p.windows],
    ["Mac", p.mac],
    ["Linux", p.linux],
  ].filter(([, on]) => on);
  if (supported.length === 0) return <Muted>–</Muted>;
  return (
    <span className="flex gap-1 text-xs">
      {supported.map(([label]) => (
        <span key={String(label)} className="rounded border border-line-strong px-1.5 py-px text-[11px] text-muted">
          {label}
        </span>
      ))}
    </span>
  );
}

function Muted({ children }: { children: React.ReactNode }) {
  return <span className="text-faint">{children}</span>;
}

export function formatHours(minutes: number) {
  if (minutes < 60) return `${minutes} min`;
  const h = minutes / 60;
  return `${h < 10 ? h.toFixed(1) : Math.round(h).toLocaleString()} h`;
}

/** Text colour for a positive-review percentage. */
export function reviewColor(pct: number) {
  return pct >= 80
    ? "text-review-great"
    : pct >= 70
      ? "text-review-good"
      : pct >= 40
        ? "text-review-mixed"
        : "text-review-bad";
}
