"use client";

import { useState } from "react";
import { Check, Copy, Sparkles } from "lucide-react";
import { Button } from "@/components/ui/button";

interface ContentEditorProps {
  productId: string;
  productTitle: string;
  kind: string;
  content: string;
}

export function ContentEditor({ productTitle, kind, content: initialContent }: ContentEditorProps) {
  const [content, setContent] = useState(initialContent);
  const [copied, setCopied] = useState(false);

  async function handleCopy() {
    try {
      await navigator.clipboard.writeText(content);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      // Clipboard API not available — silently ignore
    }
  }

  const kindLabel: Record<string, string> = {
    DESCRIPTION: "Product Description",
    SEO: "SEO Metadata",
    EMAIL: "Email Copy",
  };

  return (
    <div className="rounded-lg border bg-card">
      {/* Header */}
      <div className="flex items-center justify-between border-b px-4 py-2.5">
        <div className="flex items-center gap-2 min-w-0">
          <p className="truncate text-sm font-medium">{productTitle}</p>
          <span className="shrink-0 rounded-full border border-primary/30 bg-primary/5 px-2 py-0.5 text-xs text-primary flex items-center gap-1">
            <Sparkles className="h-2.5 w-2.5" />
            AI-generated
          </span>
          <span className="shrink-0 text-xs text-muted-foreground">{kindLabel[kind] ?? kind}</span>
        </div>
        <Button
          variant="ghost"
          size="icon-sm"
          onClick={handleCopy}
          aria-label="Copy to clipboard"
        >
          {copied ? (
            <Check className="h-3.5 w-3.5 text-green-500" />
          ) : (
            <Copy className="h-3.5 w-3.5" />
          )}
        </Button>
      </div>

      {/* Editable content */}
      <textarea
        value={content}
        onChange={(e) => setContent(e.target.value)}
        rows={6}
        className="w-full resize-y rounded-b-lg bg-transparent px-4 py-3 font-mono text-xs leading-relaxed focus-visible:outline-none"
        aria-label={`Generated ${kindLabel[kind] ?? kind} for ${productTitle}`}
      />
    </div>
  );
}
