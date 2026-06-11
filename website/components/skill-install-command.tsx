"use client";

import { useState } from "react";
import { Check, Copy } from "lucide-react";
import { cn } from "@/lib/utils";

const COMMAND = "npx skills add tracewayapp/traceway";

export function SkillInstallCommand({ className }: { className?: string }) {
  const [copied, setCopied] = useState(false);

  async function copyCommand() {
    try {
      await navigator.clipboard.writeText(COMMAND);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      setCopied(false);
    }
  }

  return (
    <button
      type="button"
      onClick={copyCommand}
      className={cn(
        "group inline-flex max-w-full items-center gap-3 rounded-lg border border-hair-2 bg-ink-1 py-2.5 pl-4 pr-2.5 text-left font-mono text-xs sm:text-[0.8125rem] transition-colors hover:bg-ink-2",
        className
      )}
    >
      <span className="text-a2" aria-hidden>
        $
      </span>
      <span className="truncate text-fg-0">{COMMAND}</span>
      <span
        className={cn(
          "grid size-7 shrink-0 place-items-center rounded-md border border-hair bg-ink-2 transition-colors",
          copied ? "text-ok" : "text-fg-2 group-hover:text-fg-0"
        )}
        aria-hidden
      >
        {copied ? <Check className="size-3.5" /> : <Copy className="size-3.5" />}
      </span>
      <span className="sr-only">
        {copied ? "Copied to clipboard" : "Copy install command"}
      </span>
    </button>
  );
}
