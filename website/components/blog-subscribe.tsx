"use client";

import { useState, FormEvent } from "react";
import { Eyebrow } from "@/components/eyebrow";

export function BlogSubscribe() {
  const [email, setEmail] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [status, setStatus] = useState<"idle" | "success" | "error">("idle");

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setSubmitting(true);
    setStatus("idle");

    try {
      const res = await fetch("/api/subscribe", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ email: email.trim() }),
      });
      setStatus(res.ok ? "success" : "error");
    } catch {
      setStatus("error");
    } finally {
      setSubmitting(false);
    }
  }

  const inputStyle: React.CSSProperties = {
    background: "color-mix(in oklab, var(--ink-0) 60%, transparent)",
    border: "1px solid var(--hair-2)",
    color: "var(--fg-0)",
    fontFamily: "var(--font-mono)",
  };

  return (
    <div
      className="mt-16 rounded-[14px] p-6 md:p-8"
      style={{
        background: "var(--ink-1)",
        border: "1px solid var(--hair-2)",
      }}
    >
      <Eyebrow>Subscribe</Eyebrow>
      <h3
        className="mt-3 mb-2 text-[20px] font-medium"
        style={{ fontFamily: "var(--font-body)", color: "var(--fg-0)" }}
      >
        Get new engineering posts in your inbox
      </h3>

      {status === "success" ? (
        <p
          className="mt-3 text-[14px]"
          style={{ color: "var(--fg-2)", fontFamily: "var(--font-mono)" }}
        >
          Thanks, we&apos;ll email you when we publish.
        </p>
      ) : (
        <form
          onSubmit={handleSubmit}
          className="mt-4 flex flex-col sm:flex-row gap-3"
        >
          <input
            type="email"
            required
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            placeholder="you@company.com"
            aria-label="Email address"
            className="flex-1 px-4 py-3 rounded-md text-[15px] outline-none focus:border-[color:var(--a1)] transition-colors"
            style={inputStyle}
          />
          <button
            type="submit"
            disabled={submitting}
            className="btn btn-accent px-8 py-3 justify-center disabled:opacity-60 disabled:cursor-not-allowed"
          >
            {submitting ? "Subscribing…" : "Subscribe"}
          </button>
        </form>
      )}

      {status === "error" && (
        <p
          className="mt-3 text-[13px]"
          style={{ color: "var(--crit)", fontFamily: "var(--font-mono)" }}
        >
          Something went wrong. Please try again.
        </p>
      )}
    </div>
  );
}
