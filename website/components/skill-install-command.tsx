"use client";

import { useState } from "react";
import { Check, Copy } from "lucide-react";
import { cn } from "@/lib/utils";

const COMMAND = "npx skills add tracewayapp/traceway";

export function SkillInstallCommand({
  className,
  size = "default",
}: {
  className?: string;
  size?: "default" | "lg";
}) {
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
        "group inline-flex max-w-full items-center rounded-lg border border-hair-2 bg-ink-1 text-left font-mono transition-colors hover:bg-ink-2",
        size === "lg"
          ? "gap-3 py-3 pl-4 pr-2.5 text-xs sm:gap-4 sm:py-4 sm:pl-6 sm:pr-3.5 sm:text-base"
          : "gap-3 py-2.5 pl-4 pr-2.5 text-xs sm:text-[0.8125rem]",
        className
      )}
    >
      <span className="text-a2" aria-hidden>
        $
      </span>
      <span className="flex-1 truncate text-fg-0">{COMMAND}</span>
      <span
        className={cn(
          "grid shrink-0 place-items-center rounded-md border border-hair bg-ink-2 transition-colors",
          size === "lg" ? "size-7 sm:size-9" : "size-7",
          copied ? "text-ok" : "text-fg-2 group-hover:text-fg-0"
        )}
        aria-hidden
      >
        {copied ? (
          <Check className={size === "lg" ? "size-4" : "size-3.5"} />
        ) : (
          <Copy className={size === "lg" ? "size-4" : "size-3.5"} />
        )}
      </span>
      <span className="sr-only">
        {copied ? "Copied to clipboard" : "Copy install command"}
      </span>
    </button>
  );
}
