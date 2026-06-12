"use client";

import { useRef, KeyboardEvent } from "react";
import { Button } from "@/components/ui/button";
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
        placeholder="Ask about your store data… (Enter to send, Shift+Enter for newline)"
        className="flex-1 resize-none overflow-hidden rounded-lg border bg-background px-3 py-2 text-sm leading-relaxed focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:cursor-not-allowed disabled:opacity-50"
        onKeyDown={handleKeyDown}
        onInput={handleInput}
        aria-label="Chat question input"
        aria-multiline="true"
      />
      <Button
        onClick={submit}
        disabled={disabled}
        size="icon"
        aria-label="Send question"
      >
        <Send className="h-4 w-4" />
      </Button>
    </div>
  );
}
