"use client";

import Link from "next/link";
import { ArrowRight } from "lucide-react";

const TIERS = [
  {
    id: "starter",
    name: "Starter",
    limit: "10k",
    price: "Free",
    description: "10k issues, requests, task runs",
  },
  {
    id: "pro",
    name: "Pro",
    limit: "100k",
    price: "$12.99",
    monthlyLabel: "/ mo",
    description: "100k issues, requests, task runs",
  },
  {
    id: "premium",
    name: "Premium",
    limit: "1M",
    price: "$24.99",
    monthlyLabel: "/ mo",
    description: "1 million issues, requests, task runs",
  },
  {
    id: "enterprise",
    name: "Enterprise",
    limit: "200M",
    price: "$499.99",
    monthlyLabel: "/ mo",
    description: "200M events. $0.0000025 / event.",
    highlight: true,
  },
  {
    id: "enterprise-plus",
    name: "Enterprise+",
    limit: "Unlimited",
    price: "Contact Us",
    description:
      "Dedicated SRE, shared Slack, tailored SLAs. No data leaves your cloud.",
    badge: "Managed self-hosting",
  },
];

export function PricingCalculator() {
  return (
    <div className="w-full max-w-5xl mx-auto">
      <div
        className="rounded-[14px] overflow-hidden"
        style={{
          background: "linear-gradient(180deg, var(--ink-3), var(--ink-2))",
          border: "1px solid var(--hair-2)",
        }}
      >
        <div
          className="grid grid-cols-12 gap-4 px-6 py-3 text-[11px] font-semibold uppercase tracking-wider"
          style={{
            background: "rgba(0,0,0,0.15)",
            borderBottom: "1px solid var(--hair)",
            color: "var(--fg-3)",
            fontFamily: "var(--font-mono)",
          }}
        >
          <div className="col-span-3">Tier</div>
          <div className="col-span-3">Monthly events</div>
          <div className="col-span-3">Price</div>
          <div className="col-span-3">Includes</div>
        </div>

        <div>
          {TIERS.map((tier, i) => (
            <div
              key={tier.id}
              className="grid grid-cols-12 gap-4 px-6 py-5 items-center transition-colors hover:bg-[color:color-mix(in_oklab,var(--fg-0)_3%,transparent)]"
              style={{
                borderBottom: i < TIERS.length - 1 ? "1px solid var(--hair)" : "none",
                background: tier.highlight
                  ? "linear-gradient(90deg, color-mix(in oklab, var(--a1) 12%, transparent), transparent)"
                  : undefined,
              }}
            >
              <div
                className="col-span-3 font-semibold flex flex-col gap-1.5"
                style={{
                  color: tier.highlight ? "var(--a2)" : "var(--fg-0)",
                  fontFamily: "var(--font-display)",
                }}
              >
                <span>{tier.name}</span>
                {tier.badge ? (
                  <span
                    className="inline-flex items-center self-start gap-1 px-2 py-0.5 rounded text-[10px] font-medium uppercase tracking-wider"
                    style={{
                      fontFamily: "var(--font-mono)",
                      color: "var(--ok)",
                      background: "color-mix(in oklab, var(--ok) 12%, transparent)",
                      border: "1px solid color-mix(in oklab, var(--ok) 35%, transparent)",
                      letterSpacing: "0.08em",
                    }}
                  >
                    {tier.badge}
                  </span>
                ) : null}
              </div>
              <div
                className="col-span-3 text-sm"
                style={{ color: "var(--fg-1)", fontFamily: "var(--font-mono)" }}
              >
                {tier.limit}
              </div>
              <div
                className="col-span-3 font-semibold"
                style={{
                  color: "var(--fg-0)",
                  fontFamily: "var(--font-display)",
                }}
              >
                {tier.price}
                {tier.monthlyLabel ? (
                  <span
                    className="ml-1 text-xs font-normal"
                    style={{ color: "var(--fg-3)" }}
                  >
                    {tier.monthlyLabel}
                  </span>
                ) : null}
              </div>
              <div
                className="col-span-3 text-xs leading-relaxed"
                style={{ color: "var(--fg-2)" }}
              >
                {tier.description}
              </div>
            </div>
          ))}
        </div>

        <div
          className="px-6 py-5 text-center space-y-2"
          style={{
            borderTop: "1px solid var(--hair)",
            background: "color-mix(in oklab, var(--ok) 6%, transparent)",
          }}
        >
          <p className="text-sm" style={{ color: "var(--fg-1)" }}>
            <span style={{ color: "var(--fg-0)", fontWeight: 600 }}>No overage charges, ever.</span>{" "}
            Every plan has a fixed price. If you approach your limit, we&apos;ll notify you. Your bill
            will never increase without your approval.
          </p>
          <p className="text-xs" style={{ color: "var(--fg-3)" }}>
            Each issue, HTTP request, or background task run counts as one event toward your monthly volume.
          </p>
        </div>
      </div>

      <div className="mt-8 flex justify-center">
        <Link href="https://cloud.tracewayapp.com/register" className="btn btn-accent">
          Try for free <ArrowRight className="ml-1 h-4 w-4" />
        </Link>
      </div>
    </div>
  );
}
