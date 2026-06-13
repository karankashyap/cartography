"use client";

import { useEffect } from "react";
import { usePathname } from "next/navigation";
import { useQuery } from "@urql/next";
import { cn } from "@/lib/utils";
import { ImportButton } from "@/components/dashboard/ImportButton";
import { useActiveStore } from "@/lib/active-store";

const STORES_QUERY = `
  query StoresNav {
    stores { id name platform }
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
  const { activeStoreId, setActiveStoreId, triggerRefresh } = useActiveStore();

  const [storesResult] = useQuery({ query: STORES_QUERY });
  const stores: Store[] = storesResult.data?.stores ?? [];

  useEffect(() => {
    if (!activeStoreId && stores.length > 0) {
      setActiveStoreId(stores[0].id);
    }
  }, [activeStoreId, stores, setActiveStoreId]);

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
            <select
              value={activeStoreId ?? ""}
              onChange={(e) => setActiveStoreId(e.target.value)}
              className="rounded-lg border border-white/10 bg-white/5 backdrop-blur-sm px-3 py-1.5 text-xs focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
              aria-label="Select store"
            >
              {stores.map((s) => (
                <option key={s.id} value={s.id} className="bg-background text-foreground">
                  {s.name} ({s.platform})
                </option>
              ))}
            </select>
          )}
          {pathname === "/dashboard" && (
            <ImportButton onComplete={triggerRefresh} />
          )}
        </div>
      </div>
    </nav>
  );
}
