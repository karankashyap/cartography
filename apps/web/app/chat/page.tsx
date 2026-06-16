"use client";

import { useEffect, useRef, useState } from "react";
import { useMutation } from "@urql/next";
import { MessageSquare } from "lucide-react";
import { ChatInput } from "@/components/chat/ChatInput";
import { ChatMessage, type ChatEntry } from "@/components/chat/ChatMessage";
import { useActiveStore } from "@/lib/active-store";

const ASK_MUTATION = `
  mutation Ask($storeId: ID!, $question: String!, $history: [ChatTurn!], $provider: AIProvider) {
    ask(storeId: $storeId, question: $question, history: $history, provider: $provider) {
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

let idCounter = 0;
function nextId() {
  return String(++idCounter);
}

export default function ChatPage() {
  const { activeStoreId, aiProvider } = useActiveStore();
  const [messages, setMessages] = useState<ChatEntry[]>([]);
  const bottomRef = useRef<HTMLDivElement>(null);

  const [askResult, ask] = useMutation(ASK_MUTATION);
  const loading = askResult.fetching;

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages]);

  async function handleSubmit(question: string) {
    if (!activeStoreId || loading) return;

    const pendingId = nextId();
    setMessages((prev) => [
      ...prev,
      {
        id: pendingId,
        question,
        sql: "",
        blocked: false,
        columns: [],
        rows: [],
        explanation: "",
        pending: true,
      },
    ]);

    // Build history from the last 5 successful (non-blocked, non-pending) turns.
    // These are sent as prior user/assistant context so the model can handle
    // follow-up questions without the schema being re-included in every user message.
    const history = messages
      .filter((m) => !m.pending && !m.blocked && m.sql)
      .slice(-5)
      .map((m) => ({ question: m.question, sql: m.sql }));

    const result = await ask({ storeId: activeStoreId, question, history, provider: aiProvider });

    setMessages((prev) => {
      const rest = prev.filter((m) => m.id !== pendingId);
      if (result.error) {
        return [
          ...rest,
          {
            id: nextId(),
            question,
            sql: "",
            blocked: true,
            blockReason: result.error.message,
            columns: [],
            rows: [],
            explanation: "",
          },
        ];
      }
      const data = result.data?.ask;
      if (data) {
        return [
          ...rest,
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
        ];
      }
      return rest;
    });
  }

  return (
    <main className="relative flex h-[calc(100dvh-49px)] flex-col overflow-hidden">
      {/* Scrollable message area — extra bottom padding clears the floating input */}
      <div className="flex-1 overflow-y-auto px-6 pt-6 pb-36">
        {!activeStoreId ? (
          <div className="flex h-full flex-col items-center justify-center gap-3 text-center">
            <MessageSquare className="h-8 w-8 text-muted-foreground" />
            <p className="text-sm text-muted-foreground">
              Import a store from the dashboard to start chatting
            </p>
          </div>
        ) : messages.length === 0 ? (
          <div className="flex h-full flex-col items-center justify-center gap-5 text-center">
            <MessageSquare className="h-8 w-8 text-primary/60" />
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
                  className="rounded-full border border-white/10 bg-white/5 backdrop-blur-sm px-4 py-1.5 text-xs hover:bg-white/10 disabled:opacity-40 transition-all"
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
            <div ref={bottomRef} />
          </div>
        )}
      </div>

      {/* Floating input — gradient fade behind it */}
      <div className="absolute bottom-0 inset-x-0 px-6 pb-6 pt-16 bg-gradient-to-t from-background via-background/95 to-transparent pointer-events-none">
        <div className="mx-auto max-w-3xl pointer-events-auto">
          <ChatInput onSubmit={handleSubmit} disabled={loading || !activeStoreId} />
        </div>
      </div>
    </main>
  );
}
