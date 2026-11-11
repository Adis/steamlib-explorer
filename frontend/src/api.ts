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
