"use client";

import { useEffect, useRef, useState } from "react";
import { useQuery, useMutation } from "@urql/next";
import { MessageSquare } from "lucide-react";
import { ChatInput } from "@/components/chat/ChatInput";
import { ChatMessage, type ChatEntry } from "@/components/chat/ChatMessage";

const STORES_QUERY = `
  query Stores {
    stores { id name platform }
  }
`;

const ASK_MUTATION = `
  mutation Ask($storeId: ID!, $question: String!) {
    ask(storeId: $storeId, question: $question) {
      question
      sql
      blocked
      blockReason
      columns
      rows
      explanation
    }
  }
`;

interface Store {
  id: string;
  name: string;
  platform: string;
}

let idCounter = 0;
function nextId() {
  return String(++idCounter);
}

export default function ChatPage() {
  const [selectedStoreId, setSelectedStoreId] = useState<string | null>(null);
  const [messages, setMessages] = useState<ChatEntry[]>([]);
  const bottomRef = useRef<HTMLDivElement>(null);

  const [storesResult] = useQuery({ query: STORES_QUERY });
  const stores: Store[] = storesResult.data?.stores ?? [];
  const activeStoreId = selectedStoreId ?? stores[0]?.id ?? null;

  const [askResult, ask] = useMutation(ASK_MUTATION);
  const loading = askResult.fetching;

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages]);

  async function handleSubmit(question: string) {
    if (!activeStoreId) return;

    const result = await ask({ storeId: activeStoreId, question });

    if (result.error) {
      setMessages((prev) => [
        ...prev,
        {
          id: nextId(),
          question,
          sql: "",
          blocked: true,
          blockReason: result.error!.message,
          columns: [],
          rows: [],
          explanation: "",
        },
      ]);
      return;
    }

    const data = result.data?.ask;
    if (data) {
      setMessages((prev) => [
        ...prev,
        {
          id: nextId(),
          question: data.question,
          sql: data.sql ?? "",
          blocked: data.blocked,
          blockReason: data.blockReason,
          columns: data.columns ?? [],
          rows: data.rows ?? [],
          explanation: data.explanation ?? "",
        },
      ]);
    }
  }

  return (
    <main className="flex h-screen flex-col bg-background">
      {/* Header */}
      <div className="flex shrink-0 items-center justify-between border-b px-6 py-4">
        <div>
          <h1 className="text-lg font-semibold tracking-tight">Chat</h1>
          <p className="text-xs text-muted-foreground">
            Ask questions about your store in plain English
          </p>
        </div>
        {stores.length > 0 && (
          <select
            value={activeStoreId ?? ""}
            onChange={(e) => setSelectedStoreId(e.target.value)}
            className="rounded-md border bg-background px-3 py-1.5 text-sm"
            aria-label="Select store"
          >
            {stores.map((s) => (
              <option key={s.id} value={s.id}>
                {s.name} ({s.platform})
              </option>
            ))}
          </select>
        )}
      </div>

      {/* Message list */}
      <div className="flex-1 overflow-y-auto px-6 py-4">
        {!activeStoreId ? (
          <div className="flex h-full flex-col items-center justify-center gap-3 text-center">
            <MessageSquare className="h-8 w-8 text-muted-foreground" />
            <p className="text-sm text-muted-foreground">
              Import a store from the dashboard to start chatting
            </p>
          </div>
        ) : messages.length === 0 ? (
          <div className="flex h-full flex-col items-center justify-center gap-4 text-center">
            <MessageSquare className="h-8 w-8 text-muted-foreground" />
            <div className="space-y-1">
              <p className="text-sm font-medium">Ask anything about your data</p>
              <p className="text-xs text-muted-foreground">
                Try &quot;Top 5 products by revenue&quot; or &quot;How many orders last month?&quot;
              </p>
            </div>
            <div className="flex flex-wrap justify-center gap-2">
              {[
                "Top 5 products by revenue",
                "How many orders this month?",
                "Which customers spent the most?",
                "Show dead stock variants",
              ].map((suggestion) => (
                <button
                  key={suggestion}
                  onClick={() => handleSubmit(suggestion)}
                  disabled={loading}
                  className="rounded-full border px-3 py-1 text-xs hover:bg-muted disabled:opacity-50"
                >
                  {suggestion}
                </button>
              ))}
            </div>
          </div>
        ) : (
          <div className="mx-auto max-w-3xl space-y-6">
            {messages.map((msg) => (
              <ChatMessage key={msg.id} entry={msg} />
            ))}
            {loading && (
              <div className="flex justify-start">
                <div className="flex items-center gap-2 rounded-lg border bg-card px-4 py-3 text-sm text-muted-foreground">
                  <span className="flex gap-1">
                    <span className="animate-bounce [animation-delay:0ms]">·</span>
                    <span className="animate-bounce [animation-delay:150ms]">·</span>
                    <span className="animate-bounce [animation-delay:300ms]">·</span>
                  </span>
                  Thinking…
                </div>
              </div>
            )}
            <div ref={bottomRef} />
          </div>
        )}
      </div>

      {/* Input */}
      <div className="shrink-0 border-t px-6 py-4">
        <div className="mx-auto max-w-3xl">
          <ChatInput onSubmit={handleSubmit} disabled={loading || !activeStoreId} />
          <p className="mt-1.5 text-center text-xs text-muted-foreground">
            Queries run read-only against your local database.
          </p>
        </div>
      </div>
    </main>
  );
}
