"use client";

import { useEffect, useRef, useState } from "react";
import { Check, Link2 } from "lucide-react";

import { cn } from "@/lib/utils";

export function SectionShareLink({
  id,
  className,
}: {
  id: string;
  className?: string;
}) {
  const [copied, setCopied] = useState(false);
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(
    () => () => {
      if (timer.current) clearTimeout(timer.current);
    },
    [],
  );

  async function share() {
    const url = `${location.origin}${location.pathname}#${id}`;
    history.replaceState(null, "", `#${id}`);
    // Phones and tablets get the system share sheet; a cancelled sheet is
    // not an error, and anything else falls back to the clipboard.
    if (navigator.share && matchMedia("(pointer: coarse)").matches) {
      try {
        await navigator.share({ title: document.title, url });
        return;
      } catch (e) {
        if (e instanceof DOMException && e.name === "AbortError") return;
      }
    }
    try {
      await navigator.clipboard.writeText(url);
    } catch {
      return;
    }
    setCopied(true);
    if (timer.current) clearTimeout(timer.current);
    timer.current = setTimeout(() => setCopied(false), 2000);
  }

  return (
    <button
      type="button"
      onClick={share}
      aria-label={copied ? "Link copied" : "Copy a link to this section"}
      className={cn(
        "inline-flex h-7 items-center gap-1.5 rounded-md border border-hair-2 px-2.5 font-mono text-[0.6875rem] font-medium tracking-[0.1em] uppercase transition-colors outline-none focus-visible:ring-[3px] focus-visible:ring-[color:var(--ring)]",
        copied ? "" : "text-fg-2 hover:border-fg-3 hover:text-fg-0",
        className,
      )}
      style={
        copied
          ? {
              color: "var(--ok)",
              borderColor: "color-mix(in oklab, var(--ok) 50%, transparent)",
            }
          : undefined
      }
    >
      {copied ? <Check className="size-3.5" /> : <Link2 className="size-3.5" />}
      {copied ? "Link copied" : "Share"}
    </button>
  );
}
