"use client";

import { useState, FormEvent } from "react";
import Link from "next/link";
import { GITHUB_URL, DISCORD_URL } from "@/lib/links";

export function HeroEmailCTA({
  placeholder = "Email Address",
}: {
  placeholder?: string;
}) {
  const [email, setEmail] = useState("");

  function handleSubmit(e: FormEvent) {
    e.preventDefault();
    const trimmed = email.trim();
    const base = "https://cloud.tracewayapp.com/register";
    const url = trimmed ? `${base}?email=${encodeURIComponent(trimmed)}` : base;
    window.location.href = url;
  }

  return (
    <div className="w-full max-w-xl mx-auto">
      <form
        onSubmit={handleSubmit}
        className="flex items-stretch gap-1 rounded-[12px] p-1.5"
        style={{
          background: "color-mix(in oklab, var(--ink-2) 85%, transparent)",
          border: "1px solid var(--hair-2)",
          boxShadow: "inset 0 1px 0 rgba(255,255,255,0.03)",
        }}
      >
        <input
          type="email"
          required
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          placeholder={placeholder}
          aria-label="Email address"
          className="flex-1 min-w-0 px-3 sm:px-4 py-2.5 rounded-md text-[14px] sm:text-[15px] bg-transparent outline-none"
          style={{
            color: "var(--fg-0)",
            fontFamily: "var(--font-mono)",
          }}
        />
        <button
          type="submit"
          className="btn btn-accent whitespace-nowrap shrink-0 px-3 sm:px-5"
          style={{ borderRadius: 8 }}
        >
          Start for free
        </button>
      </form>
      <p
        className="mt-3 text-center text-[13px]"
        style={{ color: "var(--fg-2)", fontFamily: "var(--font-mono)" }}
      >
        <Link
          href={GITHUB_URL}
          target="_blank"
          rel="noopener noreferrer"
          className="underline underline-offset-4 hover:text-[color:var(--fg-0)] transition-colors"
          style={{ color: "var(--fg-0)" }}
        >
          Star on GitHub
        </Link>{" "}
        ·{" "}
        <Link
          href={DISCORD_URL}
          target="_blank"
          rel="noopener noreferrer"
          className="underline underline-offset-4 hover:text-[color:var(--fg-0)] transition-colors"
          style={{ color: "var(--fg-0)" }}
        >
          Join the Discord
        </Link>
      </p>
    </div>
  );
}
