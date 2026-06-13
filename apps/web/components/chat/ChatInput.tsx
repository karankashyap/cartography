"use client";

import { useRef, KeyboardEvent } from "react";
import { Send } from "lucide-react";

interface ChatInputProps {
  onSubmit: (question: string) => void;
  disabled?: boolean;
}

export function ChatInput({ onSubmit, disabled }: ChatInputProps) {
  const ref = useRef<HTMLTextAreaElement>(null);

  function submit() {
    const val = ref.current?.value.trim();
    if (!val || disabled) return;
    onSubmit(val);
    if (ref.current) {
      ref.current.value = "";
      ref.current.style.height = "auto";
    }
  }

  function handleKeyDown(e: KeyboardEvent<HTMLTextAreaElement>) {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      submit();
    }
  }

  function handleInput() {
    const el = ref.current;
    if (!el) return;
    el.style.height = "auto";
    el.style.height = `${Math.min(el.scrollHeight, 120)}px`;
  }

  return (
    <div className="flex items-end gap-2">
      <textarea
        ref={ref}
        rows={1}
        disabled={disabled}
        placeholder="Ask about your store data… (Enter to send)"
        className="flex-1 resize-none overflow-hidden rounded-xl border border-white/10 bg-white/5 backdrop-blur-sm px-4 py-2.5 text-sm leading-relaxed placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/60 disabled:cursor-not-allowed disabled:opacity-40 transition-all"
        onKeyDown={handleKeyDown}
        onInput={handleInput}
        aria-label="Chat question input"
        aria-multiline="true"
      />
      <button
        onClick={submit}
        disabled={disabled}
        aria-label="Send question"
        className="shrink-0 flex h-10 w-10 items-center justify-center rounded-xl bg-primary text-primary-foreground transition-all hover:bg-primary/80 disabled:opacity-40 disabled:cursor-not-allowed"
      >
        <Send className="h-4 w-4" />
      </button>
    </div>
  );
}
