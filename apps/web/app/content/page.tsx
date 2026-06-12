"use client";

import { useState } from "react";
import { useQuery, useMutation } from "@urql/next";
import { Loader2, Wand2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { ProductPicker } from "@/components/content/ProductPicker";
import { ContentEditor } from "@/components/content/ContentEditor";

const STORES_QUERY = `
  query Stores {
    stores { id name platform }
  }
`;

const PRODUCTS_BY_IDS_QUERY = `
  query ProductsByIds($storeId: ID!, $ids: [String!]!) {
    searchProducts(storeId: $storeId, query: "", limit: 200) {
      id
      title
    }
  }
`;

const GENERATE_MUTATION = `
  mutation GenerateContent($productIds: [ID!]!, $kind: ContentKind!) {
    generateContent(productIds: $productIds, kind: $kind) {
      productId
      kind
      content
    }
  }
`;

type ContentKind = "DESCRIPTION" | "SEO" | "EMAIL";

interface Store {
  id: string;
  name: string;
  platform: string;
}

interface GeneratedContent {
  productId: string;
  kind: ContentKind;
  content: string;
}

const KIND_LABELS: Record<ContentKind, string> = {
  DESCRIPTION: "Product Description",
  SEO: "SEO Metadata",
  EMAIL: "Email Copy",
};

export default function ContentPage() {
  const [selectedStoreId, setSelectedStoreId] = useState<string | null>(null);
  const [selectedIds, setSelectedIds] = useState<string[]>([]);
  const [kind, setKind] = useState<ContentKind>("DESCRIPTION");
  const [results, setResults] = useState<GeneratedContent[]>([]);

  const [storesResult] = useQuery({ query: STORES_QUERY });
  const stores: Store[] = storesResult.data?.stores ?? [];
  const activeStoreId = selectedStoreId ?? stores[0]?.id ?? null;

  // Keep a title map so ContentEditor can show the product name
  const [allProductsResult] = useQuery({
    query: PRODUCTS_BY_IDS_QUERY,
    variables: { storeId: activeStoreId, ids: selectedIds },
    pause: !activeStoreId,
  });
  const titleMap: Record<string, string> = Object.fromEntries(
    (allProductsResult.data?.searchProducts ?? []).map(
      (p: { id: string; title: string }) => [p.id, p.title]
    )
  );

  const [generateResult, generate] = useMutation(GENERATE_MUTATION);
  const loading = generateResult.fetching;

  async function handleGenerate() {
    if (selectedIds.length === 0 || loading) return;
    const result = await generate({ productIds: selectedIds, kind });
    if (result.data?.generateContent) {
      setResults(result.data.generateContent as GeneratedContent[]);
    }
  }

  return (
    <main className="min-h-screen bg-background">
      <div className="mx-auto max-w-5xl px-6 py-8">
        {/* Header */}
        <div className="mb-8">
          <h1 className="text-2xl font-bold tracking-tight">Content Studio</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            Generate product descriptions, SEO metadata, and email copy with AI
          </p>
        </div>

        <div className="grid gap-8 lg:grid-cols-[340px_1fr]">
          {/* Left panel — controls */}
          <div className="space-y-6">
            {/* Store selector */}
            {stores.length > 0 && (
              <div className="space-y-1.5">
                <label className="text-sm font-medium" htmlFor="store-select">
                  Store
                </label>
                <select
                  id="store-select"
                  value={activeStoreId ?? ""}
                  onChange={(e) => {
                    setSelectedStoreId(e.target.value);
                    setSelectedIds([]);
                    setResults([]);
                  }}
                  className="w-full rounded-md border bg-background px-3 py-1.5 text-sm"
                >
                  {stores.map((s) => (
                    <option key={s.id} value={s.id}>
                      {s.name} ({s.platform})
                    </option>
                  ))}
                </select>
              </div>
            )}

            {/* Content kind */}
            <div className="space-y-1.5">
              <p className="text-sm font-medium">Content type</p>
              <div className="flex gap-2">
                {(["DESCRIPTION", "SEO", "EMAIL"] as ContentKind[]).map((k) => (
                  <button
                    key={k}
                    onClick={() => setKind(k)}
                    className={`flex-1 rounded-md border px-2 py-1.5 text-xs font-medium transition-colors ${
                      kind === k
                        ? "bg-primary text-primary-foreground border-primary"
                        : "bg-background hover:bg-muted"
                    }`}
                  >
                    {KIND_LABELS[k].split(" ")[0]}
                  </button>
                ))}
              </div>
            </div>

            {/* Product picker */}
            {activeStoreId && (
              <div className="space-y-1.5">
                <p className="text-sm font-medium">Products</p>
                <ProductPicker
                  storeId={activeStoreId}
                  selected={selectedIds}
                  onChange={setSelectedIds}
                />
              </div>
            )}

            {/* Generate button */}
            <Button
              className="w-full"
              onClick={handleGenerate}
              disabled={selectedIds.length === 0 || loading || !activeStoreId}
            >
              {loading ? (
                <>
                  <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                  Generating…
                </>
              ) : (
                <>
                  <Wand2 className="mr-2 h-4 w-4" />
                  Generate {KIND_LABELS[kind]}
                </>
              )}
            </Button>

            {generateResult.error && (
              <p className="text-xs text-destructive">
                {generateResult.error.message}
              </p>
            )}
          </div>

          {/* Right panel — results */}
          <div>
            {results.length === 0 ? (
              <div className="flex h-full min-h-64 flex-col items-center justify-center gap-3 rounded-lg border border-dashed text-center">
                <Wand2 className="h-8 w-8 text-muted-foreground" />
                <div className="space-y-1">
                  <p className="text-sm font-medium">No content generated yet</p>
                  <p className="text-xs text-muted-foreground">
                    Select products and click Generate
                  </p>
                </div>
              </div>
            ) : (
              <div className="space-y-4">
                <p className="text-sm text-muted-foreground">
                  {results.length} result{results.length !== 1 ? "s" : ""} ·{" "}
                  {KIND_LABELS[kind]}
                </p>
                {results.map((r) => (
                  <ContentEditor
                    key={r.productId}
                    productId={r.productId}
                    productTitle={titleMap[r.productId] ?? r.productId}
                    kind={r.kind}
                    content={r.content}
                  />
                ))}
              </div>
            )}
          </div>
        </div>
      </div>
    </main>
  );
}
