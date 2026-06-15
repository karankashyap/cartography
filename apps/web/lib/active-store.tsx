"use client";

import { createContext, useCallback, useContext, useEffect, useState } from "react";

const STORAGE_KEY = "cartograph:activeStoreId";
const PROVIDER_KEY = "cartograph:aiProvider";

export type AIProvider = "OLLAMA" | "LMSTUDIO";

interface ActiveStoreContextValue {
  activeStoreId: string | null;
  setActiveStoreId: (id: string) => void;
  refreshCount: number;
  triggerRefresh: () => void;
  aiProvider: AIProvider;
  setAIProvider: (p: AIProvider) => void;
}

const ActiveStoreContext = createContext<ActiveStoreContextValue>({
  activeStoreId: null,
  setActiveStoreId: () => {},
  refreshCount: 0,
  triggerRefresh: () => {},
  aiProvider: "OLLAMA",
  setAIProvider: () => {},
});

export function ActiveStoreProvider({ children }: { children: React.ReactNode }) {
  const [activeStoreId, _setActiveStoreId] = useState<string | null>(null);
  const [refreshCount, setRefreshCount] = useState(0);
  const [aiProvider, _setAIProvider] = useState<AIProvider>("OLLAMA");

  useEffect(() => {
    const saved = localStorage.getItem(STORAGE_KEY);
    if (saved) _setActiveStoreId(saved);
    const savedProvider = localStorage.getItem(PROVIDER_KEY) as AIProvider | null;
    if (savedProvider) _setAIProvider(savedProvider);
  }, []);

  const setActiveStoreId = useCallback((id: string) => {
    _setActiveStoreId(id);
    if (id) localStorage.setItem(STORAGE_KEY, id);
    else localStorage.removeItem(STORAGE_KEY);
  }, []);

  const setAIProvider = useCallback((p: AIProvider) => {
    _setAIProvider(p);
    localStorage.setItem(PROVIDER_KEY, p);
  }, []);

  const triggerRefresh = useCallback(() => setRefreshCount((n) => n + 1), []);

  return (
    <ActiveStoreContext.Provider
      value={{ activeStoreId, setActiveStoreId, refreshCount, triggerRefresh, aiProvider, setAIProvider }}
    >
      {children}
    </ActiveStoreContext.Provider>
  );
}

export function useActiveStore() {
  return useContext(ActiveStoreContext);
}
