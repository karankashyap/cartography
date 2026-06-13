"use client";

import { motion } from "framer-motion";
import { AlertTriangle, ChevronDown } from "lucide-react";
import { ResultTable } from "./ResultTable";

export interface ChatEntry {
  id: string;
  question: string;
  sql: string;
  blocked: boolean;
  blockReason?: string | null;
  columns: string[];
  rows: (string | null)[][];
  explanation: string;
  pending?: boolean;
}

interface ChatMessageProps {
  entry: ChatEntry;
}

export function ChatMessage({ entry }: ChatMessageProps) {
  return (
    <motion.div
      initial={{ opacity: 0, y: 10 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.2 }}
      className="space-y-3"
    >
      {/* Question bubble */}
      <div className="flex justify-end">
        <div className="max-w-[80%] rounded-2xl rounded-tr-sm bg-primary px-4 py-2.5 text-sm text-primary-foreground">
          {entry.question}
        </div>
      </div>

      {/* Answer */}
      {entry.pending ? (
        <div className="max-w-[90%]">
          <div className="inline-flex items-center gap-2 rounded-xl border border-white/10 bg-white/5 backdrop-blur-sm px-4 py-3">
            <span className="flex gap-1 text-primary">
              <span className="animate-bounce [animation-delay:0ms] text-base leading-none">▪</span>
              <span className="animate-bounce [animation-delay:150ms] text-base leading-none">▪</span>
              <span className="animate-bounce [animation-delay:300ms] text-base leading-none">▪</span>
            </span>
            <span className="text-xs text-muted-foreground">Thinking…</span>
          </div>
        </div>
      ) : (
        <div className="max-w-[90%]">
          {entry.blocked ? (
            <div className="flex items-start gap-2 rounded-xl border border-destructive/30 bg-destructive/10 backdrop-blur-sm px-4 py-3">
              <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0 text-destructive" />
              <div className="space-y-1">
                <p className="text-sm font-medium text-destructive">
                  Query not allowed
                </p>
                {entry.blockReason && (
                  <p className="text-xs text-muted-foreground">
                    {entry.blockReason}
                  </p>
                )}
              </div>
            </div>
          ) : (
            <div className="space-y-3 rounded-xl border border-white/10 bg-white/5 backdrop-blur-sm px-4 py-3">
              <p className="text-sm">{entry.explanation}</p>

              {entry.columns.length > 0 && (
                <ResultTable columns={entry.columns} rows={entry.rows} />
              )}

              {entry.sql && (
                <details className="group">
                  <summary className="flex cursor-pointer items-center gap-1 text-xs text-muted-foreground hover:text-foreground">
                    <ChevronDown className="h-3 w-3 transition-transform group-open:rotate-180" />
                    View SQL
                  </summary>
                  <pre className="mt-2 overflow-x-auto rounded-lg border border-white/10 bg-black/30 px-3 py-2 font-mono text-xs text-primary/90">
                    {entry.sql}
                  </pre>
                </details>
              )}
            </div>
          )}
        </div>
      )}
    </motion.div>
  );
}
