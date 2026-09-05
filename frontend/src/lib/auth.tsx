"use client";

// Authentication context: holds the current user, exposes login/register/logout,
// and hydrates from a persisted token on mount. Persistence and refresh live in
// the api client; this context is the React-facing surface.

import { createContext, useContext, useEffect, useState, ReactNode, useCallback } from "react";
import { api, tokenStore } from "./api";
import type { AuthResponse, User } from "./types";

interface AuthState {
  user: User | null;
  loading: boolean;
  login: (email: string, password: string) => Promise<void>;
  loginWithGoogle: (idToken: string) => Promise<void>;
  register: (input: RegisterInput) => Promise<void>;
  logout: () => Promise<void>;
}

interface RegisterInput {
  org_name: string;
  name: string;
  email: string;
  password: string;
}

const AuthContext = createContext<AuthState | undefined>(undefined);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    // Hydrate the session from a stored token, if any. `loading` is resolved on the
    // way out of the promise rather than synchronously in the no-token branch: the
    // token lives in localStorage, so it cannot seed useState without the server and
    // the browser disagreeing, and settling it here keeps both paths to one update.
    let cancelled = false;

    const hydrate = async () => {
      if (!tokenStore.access) return;
      try {
        const me = await api.get<User>("/api/v1/me");
        if (!cancelled) setUser(me);
      } catch {
        tokenStore.clear();
      }
    };

    void hydrate().finally(() => {
      if (!cancelled) setLoading(false);
    });

    return () => {
      cancelled = true;
    };
  }, []);

  const applyAuth = useCallback((res: AuthResponse) => {
    tokenStore.set(res.access_token, res.refresh_token);
    setUser(res.user);
  }, []);

  const login = useCallback(
    async (email: string, password: string) => {
      const res = await api.post<AuthResponse>("/api/v1/auth/login", { email, password }, false);
      applyAuth(res);
    },
    [applyAuth],
  );

  const loginWithGoogle = useCallback(
    async (idToken: string) => {
      // Same response shape and cookie behaviour as /auth/login, so the token
      // store + user hydrate is identical — only the credential differs.
      const res = await api.post<AuthResponse>("/api/v1/auth/google", { id_token: idToken }, false);
      applyAuth(res);
    },
    [applyAuth],
  );

  const register = useCallback(
    async (input: RegisterInput) => {
      const res = await api.post<AuthResponse>("/api/v1/auth/register", input, false);
      applyAuth(res);
    },
    [applyAuth],
  );

  const logout = useCallback(async () => {
    const refresh = tokenStore.refresh;
    if (refresh) {
      await api.post("/api/v1/auth/logout", { refresh_token: refresh }, false).catch(() => {});
    }
    tokenStore.clear();
    setUser(null);
  }, []);

  return (
    <AuthContext.Provider value={{ user, loading, login, loginWithGoogle, register, logout }}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth(): AuthState {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth must be used within AuthProvider");
  return ctx;
}

// Where a "Start free" / "Get started" CTA should point. A signed-in visitor has no use
// for /register (it only bounces them back to the app, and until auth resolves that
// bounce flashes a blank screen) — send them straight to the dashboard, matching the
// nav's own CTA. One place so every marketing CTA agrees.
export function useStartHref(): string {
  const { user } = useAuth();
  return user ? "/dashboard" : "/register";
}
