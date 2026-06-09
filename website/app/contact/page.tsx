"use client";

import { useState, FormEvent } from "react";
import { ChevronDown } from "lucide-react";
import { Eyebrow } from "@/components/eyebrow";
import { getCalendlyUrl, hasCalendly } from "@/lib/calendly";

const COMPANY_SIZES = [
  "Just me",
  "2 – 10",
  "11 – 50",
  "51 – 200",
  "201 – 500",
  "500+",
];

const REGISTER_URL = "https://cloud.tracewayapp.com/register";

export default function Contact() {
  const [firstName, setFirstName] = useState("");
  const [lastName, setLastName] = useState("");
  const [email, setEmail] = useState("");
  const [companySize, setCompanySize] = useState("");
  const [submitting, setSubmitting] = useState(false);

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setSubmitting(true);

    const trimmed = email.trim();
    const payload = {
      firstName: firstName.trim(),
      lastName: lastName.trim(),
      email: trimmed,
      companySize,
    };

    try {
      await fetch("/api/contact", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload),
      });
    } catch {
      // Don't block the user if the notification fails. They still get to sign up.
    }

    // If NEXT_PUBLIC_CALENDLY_URL is set, send the user to Calendly with
    // their name + email pre-filled so they can book a 30-minute call.
    // Otherwise fall back to the cloud register page with email pre-filled.
    let url: string;
    if (hasCalendly) {
      const fullName = `${payload.firstName} ${payload.lastName}`.trim();
      url = getCalendlyUrl({ email: trimmed || undefined, name: fullName || undefined });
    } else {
      url = trimmed
        ? `${REGISTER_URL}?email=${encodeURIComponent(trimmed)}`
        : REGISTER_URL;
    }
    window.location.href = url;
  }

  const inputStyle: React.CSSProperties = {
    background: "color-mix(in oklab, var(--ink-0) 60%, transparent)",
    border: "1px solid var(--hair-2)",
    color: "var(--fg-0)",
    fontFamily: "var(--font-mono)",
  };

  const labelStyle: React.CSSProperties = {
    color: "var(--fg-0)",
    fontFamily: "var(--font-display)",
  };

  const helperStyle: React.CSSProperties = {
    color: "var(--fg-3)",
    fontFamily: "var(--font-mono)",
  };

  return (
    <main className="relative">
      <section className="hero relative overflow-hidden pt-20 pb-24">
        <div className="wrap relative z-10 max-w-3xl mx-auto">
          <div className="text-center mb-10">
            <Eyebrow>Get started</Eyebrow>
            <h1 className="mt-4">
              We&apos;d love to hear from you.{" "}
              <em>Start your free account.</em>
            </h1>
            <p
              className="mt-5 text-[17px] max-w-[560px] mx-auto"
              style={{ color: "var(--fg-2)" }}
            >
              Tell us a bit about you and your team. On submit we&apos;ll take
              you to sign-up with your email pre-filled.
            </p>
          </div>

          <form
            onSubmit={handleSubmit}
            className="rounded-[14px] p-6 md:p-8"
            style={{
              background:
                "linear-gradient(180deg, color-mix(in oklab, var(--ink-2) 80%, transparent), color-mix(in oklab, var(--ink-1) 60%, transparent))",
              border: "1px solid var(--hair-2)",
              boxShadow: "0 30px 60px -30px rgba(0, 0, 0, 0.5)",
            }}
          >
            {/* Name row */}
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              <input
                type="text"
                required
                value={firstName}
                onChange={(e) => setFirstName(e.target.value)}
                placeholder="First name"
                aria-label="First name"
                className="w-full px-4 py-3.5 rounded-md text-[15px] outline-none focus:border-[color:var(--a1)] transition-colors"
                style={inputStyle}
              />
              <input
                type="text"
                required
                value={lastName}
                onChange={(e) => setLastName(e.target.value)}
                placeholder="Last name"
                aria-label="Last name"
                className="w-full px-4 py-3.5 rounded-md text-[15px] outline-none focus:border-[color:var(--a1)] transition-colors"
                style={inputStyle}
              />
            </div>

            {/* Email */}
            <input
              type="email"
              required
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              placeholder="Email*"
              aria-label="Email address"
              className="mt-4 w-full px-4 py-3.5 rounded-md text-[15px] outline-none focus:border-[color:var(--a1)] transition-colors"
              style={inputStyle}
            />

            {/* Company size */}
            <div className="mt-8">
              <label
                htmlFor="company-size"
                className="block text-[15px] font-medium"
                style={labelStyle}
              >
                How many employees work at your company?{" "}
                <span style={{ color: "var(--crit)" }}>*</span>
              </label>
              <p className="mt-1 text-[13px]" style={helperStyle}>
                This info will help us connect you to the right person.
              </p>
              <div className="mt-3 relative">
                <select
                  id="company-size"
                  required
                  value={companySize}
                  onChange={(e) => setCompanySize(e.target.value)}
                  className="w-full appearance-none px-4 py-3.5 pr-10 rounded-md text-[15px] outline-none focus:border-[color:var(--a1)] transition-colors"
                  style={inputStyle}
                >
                  <option value="" disabled>
                    Select an option
                  </option>
                  {COMPANY_SIZES.map((c) => (
                    <option key={c} value={c}>
                      {c}
                    </option>
                  ))}
                </select>
                <ChevronDown
                  className="absolute right-3 top-1/2 -translate-y-1/2 pointer-events-none"
                  size={18}
                  style={{ color: "var(--fg-3)" }}
                />
              </div>
            </div>

            <div className="mt-8 flex justify-center">
              <button
                type="submit"
                disabled={submitting}
                className="btn btn-accent px-10 py-3 min-w-[200px] justify-center disabled:opacity-60 disabled:cursor-not-allowed"
              >
                {submitting ? "Redirecting…" : "Submit"}
              </button>
            </div>

            <p
              className="mt-5 text-center text-[12px]"
              style={{
                color: "var(--fg-3)",
                fontFamily: "var(--font-mono)",
              }}
            >
              We&apos;ll take you to sign-up with your email pre-filled.
            </p>
          </form>
        </div>
      </section>
    </main>
  );
}
