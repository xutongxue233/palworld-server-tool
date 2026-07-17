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
  passwordConfigured: boolean | null;
  passwordChangeable: boolean;
  login: (password: string) => Promise<void>;
  initializePassword: (
    password: string,
    passwordConfirmation: string,
  ) => Promise<void>;
  changePassword: (
    password: string,
    passwordConfirmation: string,
  ) => Promise<void>;
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
  const [passwordConfigured, setPasswordConfigured] = useState<boolean | null>(
    null,
  );
  const [passwordChangeable, setPasswordChangeable] = useState(true);

  const storeToken = useCallback((nextToken: string) => {
    localStorage.setItem(TOKEN_KEY, nextToken);
    setToken(nextToken);
  }, []);

  const refreshAuthStatus = useCallback(async () => {
    const status = await api.getAuthStatus();
    setPasswordConfigured(status.password_configured);
    setPasswordChangeable(status.password_changeable);
  }, []);

  const logout = useCallback(() => {
    localStorage.removeItem(TOKEN_KEY);
    setToken(null);
    window.dispatchEvent(new CustomEvent("palworld:fleet-reset"));
  }, []);

  const login = useCallback(
    async (password: string) => {
      const result = await api.login(password);
      storeToken(result.token);
    },
    [storeToken],
  );

  const initializePassword = useCallback(
    async (password: string, passwordConfirmation: string) => {
      try {
        const result = await api.initializePassword(
          password,
          passwordConfirmation,
        );
        storeToken(result.token);
        setPasswordConfigured(true);
        setPasswordChangeable(true);
      } catch (error) {
        await refreshAuthStatus().catch(() => undefined);
        throw error;
      }
    },
    [refreshAuthStatus, storeToken],
  );

  const changePassword = useCallback(
    async (password: string, passwordConfirmation: string) => {
      const result = await api.changePassword(password, passwordConfirmation);
      storeToken(result.token);
      setPasswordConfigured(true);
    },
    [storeToken],
  );

  useEffect(() => {
    let active = true;
    void api
      .getAuthStatus()
      .then((status) => {
        if (!active) return;
        setPasswordConfigured(status.password_configured);
        setPasswordChangeable(status.password_changeable);
      })
      .catch(() => {
        if (active) setPasswordConfigured(null);
      });
    return () => {
      active = false;
    };
  }, []);

  useEffect(() => {
    window.addEventListener("palworld:auth-expired", logout);
    return () => window.removeEventListener("palworld:auth-expired", logout);
  }, [logout]);

  const value = useMemo(
    () => ({
      token,
      isAuthenticated: Boolean(token),
      passwordConfigured,
      passwordChangeable,
      login,
      initializePassword,
      changePassword,
      logout,
    }),
    [
      changePassword,
      initializePassword,
      login,
      logout,
      passwordChangeable,
      passwordConfigured,
      token,
    ],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth() {
  const value = useContext(AuthContext);
  if (!value) throw new Error("useAuth must be used inside AuthProvider");
  return value;
}
