"use client";

import { useState } from "react";
import { Check, Copy } from "lucide-react";

export function DockerCommand() {
  const [copied, setCopied] = useState(false);
  const command = "docker compose up -d";

  function handleCopy() {
    navigator.clipboard.writeText(command);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  }

  return (
    <div className="relative inline-flex items-center gap-3 rounded-lg bg-zinc-900 text-zinc-100 px-5 py-3 font-mono text-sm shadow-lg">
      <span className="text-zinc-500 select-none">$</span>
      <code>{command}</code>
      <button
        onClick={handleCopy}
        className="ml-2 text-zinc-400 hover:text-white transition-colors"
        aria-label="Copy command"
      >
        {copied ? (
          <Check className="h-4 w-4 text-green-400" />
        ) : (
          <Copy className="h-4 w-4" />
        )}
      </button>
    </div>
  );
}
