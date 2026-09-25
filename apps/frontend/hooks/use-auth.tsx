"use client";

import { useRouter } from "next/navigation";
import { createContext, useCallback, useContext, useEffect, useMemo, useState } from "react";

import { authStorage } from "@/lib/auth-storage";
import { authService } from "@/services/auth";
import type { User } from "@/types";

type AuthContextValue = {
  user: User | null;
  loading: boolean;
  setUser: (user: User | null) => void;
  logout: () => Promise<void>;
};

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const router = useRouter();
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);

  // The stored user renders immediately, then /auth/me confirms the token is still valid.
  useEffect(() => {
    const stored = authStorage.getUser();
    if (stored) setUser(stored);

    if (!authStorage.getToken()) {
      setLoading(false);
      return;
    }

    let active = true;
    authService
      .me()
      .then((fresh) => {
        if (active) setUser(fresh);
      })
      .catch(() => {
        // A rejected token is already cleared by the API client interceptor.
        if (active) setUser(null);
      })
      .finally(() => {
        if (active) setLoading(false);
      });

    return () => {
      active = false;
    };
  }, []);

  const logout = useCallback(async () => {
    await authService.logout();
    setUser(null);
    router.replace("/login");
  }, [router]);

  const value = useMemo<AuthContextValue>(
    () => ({ user, loading, setUser, logout }),
    [user, loading, logout],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth(): AuthContextValue {
  const context = useContext(AuthContext);
  if (!context) {
    throw new Error("useAuth must be used inside AuthProvider");
  }
  return context;
}
