"use client";

import { useCallback, useEffect, useState } from "react";

type AsyncState<T> = {
  data: T | null;
  loading: boolean;
  error: string | null;
};

// useAsync runs a loader on mount and on every dependency change, and exposes reload so a
// mutation can refresh the view without a full page navigation.
export function useAsync<T>(loader: () => Promise<T>, deps: unknown[] = []) {
  const [state, setState] = useState<AsyncState<T>>({ data: null, loading: true, error: null });
  const [reloadToken, setReloadToken] = useState(0);

  const reload = useCallback(() => setReloadToken((token) => token + 1), []);

  useEffect(() => {
    let active = true;
    setState((current) => ({ ...current, loading: true, error: null }));

    loader()
      .then((data) => {
        if (active) setState({ data, loading: false, error: null });
      })
      .catch((error: unknown) => {
        const message = error instanceof Error ? error.message : "Terjadi kesalahan";
        // The stale data is dropped so the UI cannot show numbers from a failed refresh.
        if (active) setState({ data: null, loading: false, error: message });
      });

    return () => {
      active = false;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [...deps, reloadToken]);

  return { ...state, reload };
}
