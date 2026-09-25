import type { User } from "@/types";

const TOKEN_KEY = "tc_access_token";
const USER_KEY = "tc_user";

// The token lives in localStorage so a page reload keeps the session. It is read only in
// the browser: every accessor guards against server-side rendering.
export const authStorage = {
  getToken(): string | null {
    if (typeof window === "undefined") return null;
    return window.localStorage.getItem(TOKEN_KEY);
  },

  getUser(): User | null {
    if (typeof window === "undefined") return null;
    const raw = window.localStorage.getItem(USER_KEY);
    if (!raw) return null;
    try {
      return JSON.parse(raw) as User;
    } catch {
      // A corrupted entry would otherwise break every render that reads it.
      window.localStorage.removeItem(USER_KEY);
      return null;
    }
  },

  save(token: string, user: User): void {
    if (typeof window === "undefined") return;
    window.localStorage.setItem(TOKEN_KEY, token);
    window.localStorage.setItem(USER_KEY, JSON.stringify(user));
  },

  clear(): void {
    if (typeof window === "undefined") return;
    window.localStorage.removeItem(TOKEN_KEY);
    window.localStorage.removeItem(USER_KEY);
  },
};
