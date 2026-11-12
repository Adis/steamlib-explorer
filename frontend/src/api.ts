// Types and fetchers for the Go backend. Every call goes through Vite's proxy
// in development, so paths are relative.

export type User = {
  steam_id: string;
  persona_name: string;
  avatar_url: string;
};

export type SyncRun = {
  id: number;
  status: "running" | "ok" | "error";
  started_at: string;
  finished_at: string | null;
  games_seen: number;
  details_total: number;
  details_fetched: number;
  error: string | null;
};

export type SyncStatus = {
  run: SyncRun | null;
  stats: {
    games: number;
    with_details: number;
    with_reviews: number;
    unavailable: number;
    failed: number;
  };
};

export class ApiError extends Error {
  status: number;
  constructor(status: number, message: string) {
    super(message);
    this.status = status;
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, { credentials: "same-origin", ...init });
  if (!res.ok) {
    let message = `${res.status} ${res.statusText}`;
    try {
      const body = await res.json();
      if (body?.error) message = body.error;
    } catch {
      // not JSON
    }
    throw new ApiError(res.status, message);
  }
  if (res.status === 204) return undefined as T;
  return res.json() as Promise<T>;
}

/** The signed-in user, or null when signed out. */
export async function fetchMe(): Promise<User | null> {
  try {
    return await request<User>("/api/me");
  } catch (e) {
    if (e instanceof ApiError && e.status === 401) return null;
    throw e;
  }
}

export const fetchSyncStatus = () => request<SyncStatus>("/api/sync");

export const startSync = () =>
  request<{ run_id: number }>("/api/sync", { method: "POST" });

export const logout = () => request<void>("/auth/logout", { method: "POST" });

export type Game = {
  appid: number;
  name: string;
  store_url: string;
  header_image: string | null;
  short_description: string | null;
  release_date: string | null; // YYYY-MM-DD
  release_date_text: string | null;
  coming_soon: boolean;
  genres: string[];
  categories: string[];
  developers: string[];
  publishers: string[];
  platforms: { windows: boolean; mac: boolean; linux: boolean };
  metacritic_score: number | null;
  price_cents: number | null;
  price_currency: string | null;
  is_free: boolean;
  review_score_desc: string | null;
  review_pct: number | null;
  reviews_total: number | null;
  details_status: "pending" | "ok" | "unavailable" | "error";
  playtime_minutes: number;
  playtime_2weeks_minutes: number;
  last_played_at: string | null;
};

export type GamesPage = {
  games: Game[];
  total: number;
  limit: number;
  offset: number;
};

export type Facet = { value: string; games: number };
export type Facets = { genres: Facet[]; categories: Facet[] };

/** One page of games; params are the query string /api/games accepts. */
export const fetchGames = (params: URLSearchParams) =>
  request<GamesPage>(`/api/games?${params}`);

export const fetchFacets = () => request<Facets>("/api/facets");
