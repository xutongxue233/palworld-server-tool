import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";

import { api, TOKEN_KEY } from "@/lib/api";

interface AuthContextValue {
  token: string | null;
  isAuthenticated: boolean;
  login: (password: string) => Promise<void>;
  logout: () => void;
}

const AuthContext = createContext<AuthContextValue | null>(null);

function isTokenValid(token: string | null) {
  if (!token) return false;
  try {
    const payloadPart = token.split(".")[1];
    if (!payloadPart) return false;
    const normalized = payloadPart.replace(/-/g, "+").replace(/_/g, "/");
    const padded = normalized.padEnd(Math.ceil(normalized.length / 4) * 4, "=");
    const payload = JSON.parse(atob(padded)) as { exp?: number };
    return typeof payload.exp === "number" && payload.exp > Date.now() / 1000;
  } catch {
    return false;
  }
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [token, setToken] = useState<string | null>(() => {
    const stored = localStorage.getItem(TOKEN_KEY);
    if (isTokenValid(stored)) return stored;
    localStorage.removeItem(TOKEN_KEY);
    return null;
  });

  const logout = useCallback(() => {
    localStorage.removeItem(TOKEN_KEY);
    setToken(null);
    window.dispatchEvent(new CustomEvent("palworld:fleet-reset"));
  }, []);

  const login = useCallback(async (password: string) => {
    const result = await api.login(password);
    localStorage.setItem(TOKEN_KEY, result.token);
    setToken(result.token);
  }, []);

  useEffect(() => {
    window.addEventListener("palworld:auth-expired", logout);
    return () => window.removeEventListener("palworld:auth-expired", logout);
  }, [logout]);

  const value = useMemo(
    () => ({ token, isAuthenticated: Boolean(token), login, logout }),
    [login, logout, token],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth() {
  const value = useContext(AuthContext);
  if (!value) throw new Error("useAuth must be used inside AuthProvider");
  return value;
}
