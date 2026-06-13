"use client";

import { createContext, useCallback, useContext, useState } from "react";

interface ActiveStoreContextValue {
  activeStoreId: string | null;
  setActiveStoreId: (id: string) => void;
  refreshCount: number;
  triggerRefresh: () => void;
}

const ActiveStoreContext = createContext<ActiveStoreContextValue>({
  activeStoreId: null,
  setActiveStoreId: () => {},
  refreshCount: 0,
  triggerRefresh: () => {},
});

export function ActiveStoreProvider({ children }: { children: React.ReactNode }) {
  const [activeStoreId, setActiveStoreId] = useState<string | null>(null);
  const [refreshCount, setRefreshCount] = useState(0);
  const triggerRefresh = useCallback(() => setRefreshCount((n) => n + 1), []);

  return (
    <ActiveStoreContext.Provider
      value={{ activeStoreId, setActiveStoreId, refreshCount, triggerRefresh }}
    >
      {children}
    </ActiveStoreContext.Provider>
  );
}

export function useActiveStore() {
  return useContext(ActiveStoreContext);
}
