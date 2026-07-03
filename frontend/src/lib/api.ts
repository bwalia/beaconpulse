// Thin typed fetch client for the Beacon API. It centralizes base-URL
// resolution, bearer-token injection, transparent access-token refresh on 401,
// and error normalization into a thrown ApiRequestError.

import type { ApiError } from "./types";

// When NEXT_PUBLIC_API_BASE_URL is empty (the gateway setup), requests are made
// same-origin with relative paths (/api/...). We use ?? so an explicit empty
// string is preserved rather than falling back to a localhost default.
const BASE_URL = (process.env.NEXT_PUBLIC_API_BASE_URL ?? "http://localhost:8080").replace(/\/$/, "");

const ACCESS_KEY = "beacon.access_token";
const REFRESH_KEY = "beacon.refresh_token";

export const tokenStore = {
  get access() {
    return typeof window === "undefined" ? null : localStorage.getItem(ACCESS_KEY);
  },
  get refresh() {
    return typeof window === "undefined" ? null : localStorage.getItem(REFRESH_KEY);
  },
  set(access: string, refresh: string) {
    localStorage.setItem(ACCESS_KEY, access);
    localStorage.setItem(REFRESH_KEY, refresh);
  },
  clear() {
    localStorage.removeItem(ACCESS_KEY);
    localStorage.removeItem(REFRESH_KEY);
  },
};

export class ApiRequestError extends Error {
  code: string;
  fields?: ApiError["fields"];
  status: number;

  constructor(status: number, body: ApiError) {
    super(body.message || "Request failed");
    this.status = status;
    this.code = body.code || "internal";
    this.fields = body.fields;
  }
}

interface RequestOptions {
  method?: string;
  body?: unknown;
  auth?: boolean;
  /** Internal: prevents infinite refresh recursion. */
  _retried?: boolean;
}

async function request<T>(path: string, opts: RequestOptions = {}): Promise<T> {
  const { method = "GET", body, auth = true } = opts;
  const headers: Record<string, string> = { "Content-Type": "application/json" };

  if (auth && tokenStore.access) {
    headers["Authorization"] = `Bearer ${tokenStore.access}`;
  }

  const res = await fetch(`${BASE_URL}${path}`, {
    method,
    headers,
    body: body === undefined ? undefined : JSON.stringify(body),
  });

  // Transparently refresh once on 401, then retry the original request.
  if (res.status === 401 && auth && !opts._retried && tokenStore.refresh) {
    const refreshed = await tryRefresh();
    if (refreshed) {
      return request<T>(path, { ...opts, _retried: true });
    }
  }

  if (res.status === 204) {
    return undefined as T;
  }

  const text = await res.text();
  const json = text ? JSON.parse(text) : {};

  if (!res.ok) {
    const errBody: ApiError = json?.error ?? { code: "internal", message: "Request failed" };
    throw new ApiRequestError(res.status, errBody);
  }
  return json as T;
}

async function tryRefresh(): Promise<boolean> {
  try {
    const res = await fetch(`${BASE_URL}/api/v1/auth/refresh`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ refresh_token: tokenStore.refresh }),
    });
    if (!res.ok) {
      tokenStore.clear();
      return false;
    }
    const data = await res.json();
    tokenStore.set(data.access_token, data.refresh_token);
    return true;
  } catch {
    tokenStore.clear();
    return false;
  }
}

export const api = {
  get: <T>(path: string, auth = true) => request<T>(path, { method: "GET", auth }),
  post: <T>(path: string, body?: unknown, auth = true) =>
    request<T>(path, { method: "POST", body, auth }),
  patch: <T>(path: string, body?: unknown) => request<T>(path, { method: "PATCH", body }),
  delete: <T>(path: string) => request<T>(path, { method: "DELETE" }),
};
