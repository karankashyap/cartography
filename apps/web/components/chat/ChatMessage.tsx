"use client";

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
}

interface ChatMessageProps {
  entry: ChatEntry;
}

export function ChatMessage({ entry }: ChatMessageProps) {
  return (
    <div className="space-y-3">
      {/* Question bubble */}
      <div className="flex justify-end">
        <div className="max-w-[80%] rounded-2xl rounded-tr-sm bg-primary px-4 py-2.5 text-sm text-primary-foreground">
          {entry.question}
        </div>
      </div>

      {/* Answer */}
      <div className="max-w-[90%]">
        {entry.blocked ? (
          <div className="flex items-start gap-2 rounded-lg border border-destructive/30 bg-destructive/5 px-4 py-3">
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
          <div className="space-y-3 rounded-lg border bg-card px-4 py-3">
            {/* Explanation */}
            <p className="text-sm">{entry.explanation}</p>

            {/* Result table */}
            {entry.columns.length > 0 && (
              <ResultTable columns={entry.columns} rows={entry.rows} />
            )}

            {/* SQL disclosure */}
            {entry.sql && (
              <details className="group">
                <summary className="flex cursor-pointer items-center gap-1 text-xs text-muted-foreground hover:text-foreground">
                  <ChevronDown className="h-3 w-3 transition-transform group-open:rotate-180" />
                  View SQL
                </summary>
                <pre className="mt-2 overflow-x-auto rounded-md bg-muted px-3 py-2 font-mono text-xs">
                  {entry.sql}
                </pre>
              </details>
            )}
          </div>
        )}
      </div>
    </div>
  );
}
