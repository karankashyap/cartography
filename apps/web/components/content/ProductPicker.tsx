"use client";

import { useState } from "react";
import { useQuery } from "@urql/next";
import { Search } from "lucide-react";

const SEARCH_QUERY = `
  query SearchProducts($storeId: ID!, $query: String!, $limit: Int) {
    searchProducts(storeId: $storeId, query: $query, limit: $limit) {
      id
      title
      productType
      vendor
    }
  }
`;

interface Product {
  id: string;
  title: string;
  productType?: string | null;
  vendor?: string | null;
}

interface ProductPickerProps {
  storeId: string;
  selected: string[];
  onChange: (ids: string[]) => void;
}

export function ProductPicker({ storeId, selected, onChange }: ProductPickerProps) {
  const [searchText, setSearchText] = useState("");

  const [result] = useQuery({
    query: SEARCH_QUERY,
    variables: { storeId, query: searchText, limit: 50 },
    pause: !storeId,
  });

  const products: Product[] = result.data?.searchProducts ?? [];

  function toggle(id: string) {
    onChange(
      selected.includes(id) ? selected.filter((s) => s !== id) : [...selected, id]
    );
  }

  function selectAll() {
    onChange(products.map((p) => p.id));
  }

  function clearAll() {
    onChange([]);
  }

  return (
    <div className="flex flex-col gap-2">
      {/* Search input */}
      <div className="relative">
        <Search className="absolute left-2.5 top-2 h-3.5 w-3.5 text-muted-foreground" />
        <input
          type="search"
          value={searchText}
          onChange={(e) => setSearchText(e.target.value)}
          placeholder="Search products…"
          className="w-full rounded-md border bg-background py-1.5 pl-8 pr-3 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          aria-label="Search products"
        />
      </div>

      {/* Select/clear all */}
      <div className="flex items-center justify-between text-xs text-muted-foreground">
        <span>{selected.length} selected</span>
        <div className="flex gap-2">
          <button
            onClick={selectAll}
            disabled={products.length === 0}
            className="hover:text-foreground disabled:opacity-50"
          >
            Select all
          </button>
          <span>·</span>
          <button
            onClick={clearAll}
            disabled={selected.length === 0}
            className="hover:text-foreground disabled:opacity-50"
          >
            Clear
          </button>
        </div>
      </div>

      {/* Product list */}
      <div className="max-h-52 overflow-y-auto rounded-md border">
        {result.fetching ? (
          <p className="p-3 text-center text-sm text-muted-foreground">Loading…</p>
        ) : products.length === 0 ? (
          <p className="p-3 text-center text-sm text-muted-foreground">
            {searchText ? "No products found" : "No products in this store"}
          </p>
        ) : (
          <ul role="listbox" aria-multiselectable="true" aria-label="Product list">
            {products.map((p) => (
              <li key={p.id}>
                <label className="flex cursor-pointer items-center gap-3 px-3 py-2 hover:bg-muted">
                  <input
                    type="checkbox"
                    checked={selected.includes(p.id)}
                    onChange={() => toggle(p.id)}
                    aria-label={`Select ${p.title}`}
                    className="h-3.5 w-3.5 rounded"
                  />
                  <div className="min-w-0">
                    <p className="truncate text-sm">{p.title}</p>
                    {(p.vendor || p.productType) && (
                      <p className="truncate text-xs text-muted-foreground">
                        {[p.vendor, p.productType].filter(Boolean).join(" · ")}
                      </p>
                    )}
                  </div>
                </label>
              </li>
            ))}
          </ul>
        )}
      </div>
    </div>
  );
}
