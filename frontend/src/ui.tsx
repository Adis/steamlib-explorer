export function ErrorBox({
  children,
  className = "",
}: {
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <div
      role="alert"
      className={`rounded-md border border-warn-line bg-warn-bg px-4 py-3 text-sm text-warn-fg ${className}`}
    >
      {children}
    </div>
  );
}

// Dates are written day, short month, year ("18 Mar 2022") whatever the
// browser's locale, so 3/4/2022 is never ambiguous.
const MONTHS = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"];

export function formatDate(iso: string) {
  const d = new Date(iso);
  return `${d.getDate()} ${MONTHS[d.getMonth()]} ${d.getFullYear()}`;
}

export function formatDateTime(iso: string) {
  const d = new Date(iso);
  const hh = String(d.getHours()).padStart(2, "0");
  const mm = String(d.getMinutes()).padStart(2, "0");
  return `${formatDate(iso)}, ${hh}:${mm}`;
}

/**
 * How long ago a moment was, in the largest unit that fits: "just now",
 * "5 minutes ago", "3 hours ago", "yesterday", "4 days ago", "2 months ago",
 * "1 year ago". Units are rounded down, so "1 hour ago" means 60 to 119 minutes.
 */
export function formatTimeAgo(iso: string, now = new Date()) {
  const seconds = Math.max(0, (now.getTime() - new Date(iso).getTime()) / 1000);
  const minutes = Math.floor(seconds / 60);
  const hours = Math.floor(minutes / 60);
  const days = Math.floor(hours / 24);
  const ago = (n: number, unit: string) => `${n} ${unit}${n === 1 ? "" : "s"} ago`;
  if (minutes < 1) return "just now";
  if (hours < 1) return ago(minutes, "minute");
  if (days < 1) return ago(hours, "hour");
  if (days === 1) return "yesterday";
  if (days < 30) return ago(days, "day");
  if (days < 365) return ago(Math.floor(days / 30), "month");
  return ago(Math.floor(days / 365), "year");
}

/** How long after its release a game counts as new. */
export const NEW_RELEASE_MONTHS = 6;

/**
 * Whether the game came out on Steam within the last NEW_RELEASE_MONTHS.
 * Remasters that Steam sells as their own app (a new store page) have their
 * own, recent release date, so they count too. Games still coming soon, or
 * with a date in the future, don't.
 */
export function isNewRelease(
  game: { release_date: string | null; coming_soon: boolean },
  now = new Date(),
) {
  if (!game.release_date || game.coming_soon) return false;
  const [y, m, d] = game.release_date.split("-").map(Number);
  const released = new Date(y, m - 1, d); // local midnight, like "today"
  const today = new Date(now.getFullYear(), now.getMonth(), now.getDate());
  const cutoff = new Date(today);
  cutoff.setMonth(cutoff.getMonth() - NEW_RELEASE_MONTHS);
  return released >= cutoff && released <= today;
}

/** Small "New" pill for games released in the last six months. */
export function NewBadge({ game, className = "" }: { game: { release_date: string | null }; className?: string }) {
  const title = game.release_date ? `Released ${formatDate(game.release_date + "T00:00:00")}` : undefined;
  return (
    <span
      title={title}
      className={`new-badge label-caps inline-block rounded-md bg-accent px-1.5 py-0.5 text-on-accent ${className}`}
    >
      New<span className="sr-only"> release{title ? `, ${title.toLowerCase()}` : ""}</span>
    </span>
  );
}
