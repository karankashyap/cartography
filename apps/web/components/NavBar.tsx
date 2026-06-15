"use client";

import { useEffect, useRef, useState } from "react";
import { usePathname } from "next/navigation";
import { useQuery, useMutation } from "@urql/next";
import { ChevronDown, X } from "lucide-react";
import { cn } from "@/lib/utils";
import { ImportButton } from "@/components/dashboard/ImportButton";
import { useActiveStore, type AIProvider } from "@/lib/active-store";

const STORES_QUERY = `
  query StoresNav {
    stores { id name platform }
  }
`;

const DELETE_STORE_MUTATION = `
  mutation DeleteStore($storeId: ID!) {
    deleteStore(storeId: $storeId)
  }
`;

interface Store {
  id: string;
  name: string;
  platform: string;
}

const links = [
  { href: "/dashboard", label: "Dashboard" },
  { href: "/chat", label: "Chat" },
  { href: "/content", label: "Content" },
];

export function NavBar() {
  const pathname = usePathname();
  const { activeStoreId, setActiveStoreId, triggerRefresh, aiProvider, setAIProvider } = useActiveStore();
  const [open, setOpen] = useState(false);
  const dropdownRef = useRef<HTMLDivElement>(null);

  const [storesResult, reexecuteStores] = useQuery({ query: STORES_QUERY });
  const allStores: Store[] = storesResult.data?.stores ?? [];

  const [deletedIds, setDeletedIds] = useState<Set<string>>(new Set());
  const stores = allStores.filter((s) => !deletedIds.has(s.id));

  const [, deleteStore] = useMutation(DELETE_STORE_MUTATION);

  // Only auto-select when no store is active at all (respects localStorage restore)
  useEffect(() => {
    if (!activeStoreId && stores.length > 0) {
      setActiveStoreId(stores[0].id);
    }
  }, [activeStoreId, stores, setActiveStoreId]);

  // Close on outside click
  useEffect(() => {
    function handleClick(e: MouseEvent) {
      if (dropdownRef.current && !dropdownRef.current.contains(e.target as Node)) {
        setOpen(false);
      }
    }
    document.addEventListener("mousedown", handleClick);
    return () => document.removeEventListener("mousedown", handleClick);
  }, []);

  const activeStore = stores.find((s) => s.id === activeStoreId);

  async function handleDelete(storeId: string, e: React.MouseEvent) {
    e.stopPropagation();
    // Optimistically hide immediately so the row vanishes at once
    setDeletedIds((prev) => new Set([...prev, storeId]));
    if (storeId === activeStoreId) {
      const remaining = stores.filter((s) => s.id !== storeId);
      setActiveStoreId(remaining.length > 0 ? remaining[0].id : "");
    }
    await deleteStore({ storeId });
    reexecuteStores({ requestPolicy: "network-only" });
  }

  return (
    <nav className="shrink-0 sticky top-0 z-50 border-b border-white/10 bg-black/30 backdrop-blur-xl">
      <div className="mx-auto flex max-w-6xl items-center gap-6 px-6 py-3">
        <span className="font-semibold tracking-tight text-primary">Cartograph</span>

        <div className="flex gap-4 text-sm">
          {links.map(({ href, label }) => (
            <a
              key={href}
              href={href}
              className={cn(
                "transition-colors duration-150",
                pathname === href
                  ? "text-foreground font-medium"
                  : "text-muted-foreground hover:text-foreground"
              )}
            >
              {label}
            </a>
          ))}
        </div>

        <div className="ml-auto flex items-center gap-3">
          {stores.length > 0 && (
            <div ref={dropdownRef} className="relative">
              {/* Trigger */}
              <button
                onClick={() => setOpen((v) => !v)}
                className="flex items-center gap-1.5 rounded-lg border border-white/10 bg-white/5 backdrop-blur-sm px-3 py-1.5 text-xs hover:bg-white/10 transition-all"
              >
                <span className="max-w-[180px] truncate">
                  {activeStore ? activeStore.name : "Select store"}
                </span>
                <ChevronDown className="h-3 w-3 shrink-0 text-muted-foreground" />
              </button>

              {/* Dropdown panel */}
              {open && (
                <div className="absolute right-0 mt-1.5 w-72 rounded-xl border border-white/10 bg-black/80 backdrop-blur-xl shadow-xl z-50 py-1 overflow-hidden">
                  {stores.map((s) => (
                    <div
                      key={s.id}
                      onClick={() => { setActiveStoreId(s.id); setOpen(false); }}
                      className={cn(
                        "flex items-center justify-between gap-2 px-3 py-2 text-xs cursor-pointer transition-colors",
                        s.id === activeStoreId
                          ? "bg-white/10 text-foreground"
                          : "text-muted-foreground hover:bg-white/5 hover:text-foreground"
                      )}
                    >
                      <span className="truncate">{s.name}</span>
                      <button
                        onClick={(e) => handleDelete(s.id, e)}
                        className="shrink-0 rounded p-0.5 opacity-50 hover:opacity-100 hover:text-destructive transition-all"
                        title="Remove store"
                      >
                        <X className="h-3 w-3" />
                      </button>
                    </div>
                  ))}
                </div>
              )}
            </div>
          )}

          <select
            value={aiProvider}
            onChange={(e) => setAIProvider(e.target.value as AIProvider)}
            className="rounded-lg border border-white/10 bg-white/5 backdrop-blur-sm px-2 py-1.5 text-xs text-muted-foreground hover:bg-white/10 transition-all cursor-pointer"
            title="AI provider"
          >
            <option value="OLLAMA">Ollama</option>
            <option value="LMSTUDIO">LM Studio (Gemma 4)</option>
          </select>

          {pathname === "/dashboard" && (
            <ImportButton onComplete={triggerRefresh} />
          )}
        </div>
      </div>
    </nav>
  );
}
