"use client";

import { useState, FormEvent } from "react";

export default function Contact() {
  const [subject, setSubject] = useState("");
  const [email, setEmail] = useState("");
  const [message, setMessage] = useState("");
  const [customerType, setCustomerType] = useState("");
  const [status, setStatus] = useState<
    "idle" | "loading" | "success" | "error"
  >("idle");
  const [errorMessage, setErrorMessage] = useState("");

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setStatus("loading");
    setErrorMessage("");

    try {
      const res = await fetch("/api/contact", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ subject, email, message, customerType }),
      });

      if (!res.ok) {
        const data = await res.json();
        throw new Error(data.error || "Something went wrong");
      }

      setStatus("success");
      setSubject("");
      setEmail("");
      setMessage("");
      setCustomerType("");
    } catch (err) {
      setStatus("error");
      setErrorMessage(
        err instanceof Error ? err.message : "Something went wrong"
      );
    }
  }

  return (
    <main className="min-h-screen bg-white text-zinc-950 font-sans">
      <div className="container mx-auto px-4 max-w-xl py-20">
        <h1 className="text-4xl font-bold tracking-tight mb-2 text-zinc-900">
          Contact Us
        </h1>
        <p className="text-zinc-500 mb-10">
          Have a question or need help? Send us a message and we&apos;ll get
          back to you.
        </p>

        {status === "success" ? (
          <div className="rounded-lg border border-green-200 bg-green-50 p-6 text-center">
            <p className="text-green-800 font-medium">
              Thank you for reaching out! We&apos;ll get back to you soon.
            </p>
          </div>
        ) : (
          <form onSubmit={handleSubmit} className="space-y-5">
            <div>
              <label
                htmlFor="subject"
                className="block text-sm font-medium text-zinc-900 mb-1.5"
              >
                Subject
              </label>
              <input
                id="subject"
                type="text"
                required
                value={subject}
                onChange={(e) => setSubject(e.target.value)}
                className="w-full rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm text-zinc-900 placeholder:text-zinc-400 focus:outline-none focus:ring-2 focus:ring-zinc-900 focus:border-transparent transition-shadow"
                placeholder="What can we help with?"
              />
            </div>

            <div>
              <label
                htmlFor="email"
                className="block text-sm font-medium text-zinc-900 mb-1.5"
              >
                Email
              </label>
              <input
                id="email"
                type="email"
                required
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                className="w-full rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm text-zinc-900 placeholder:text-zinc-400 focus:outline-none focus:ring-2 focus:ring-zinc-900 focus:border-transparent transition-shadow"
                placeholder="you@example.com"
              />
            </div>

            <div>
              <label
                htmlFor="customerType"
                className="block text-sm font-medium text-zinc-900 mb-1.5"
              >
                Customer Type
              </label>
              <select
                id="customerType"
                required
                value={customerType}
                onChange={(e) => setCustomerType(e.target.value)}
                className="w-full rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm text-zinc-900 focus:outline-none focus:ring-2 focus:ring-zinc-900 focus:border-transparent transition-shadow"
              >
                <option value="" disabled>
                  Select an option
                </option>
                <option value="Existing Customer">Existing Customer</option>
                <option value="New Customer">New Customer</option>
              </select>
            </div>

            <div>
              <label
                htmlFor="message"
                className="block text-sm font-medium text-zinc-900 mb-1.5"
              >
                Message
              </label>
              <textarea
                id="message"
                required
                rows={5}
                value={message}
                onChange={(e) => setMessage(e.target.value)}
                className="w-full rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm text-zinc-900 placeholder:text-zinc-400 focus:outline-none focus:ring-2 focus:ring-zinc-900 focus:border-transparent transition-shadow resize-none"
                placeholder="Tell us more..."
              />
            </div>

            {status === "error" && (
              <div className="rounded-lg border border-red-200 bg-red-50 px-4 py-3">
                <p className="text-red-800 text-sm">{errorMessage}</p>
              </div>
            )}

            <button
              type="submit"
              disabled={status === "loading"}
              className="w-full rounded-lg bg-zinc-900 px-4 py-2.5 text-sm font-medium text-white hover:bg-zinc-800 shadow-lg shadow-zinc-900/20 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
            >
              {status === "loading" ? "Sending..." : "Send Message"}
            </button>
          </form>
        )}
      </div>
    </main>
  );
}
