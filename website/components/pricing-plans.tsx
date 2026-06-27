import Link from "next/link";
import { ArrowRight, Check, Minus } from "lucide-react";

import { getCalendlyUrl } from "@/lib/calendly";

const REGISTER_URL = "https://cloud.tracewayapp.com/register";

type Plan = {
  id: string;
  name: string;
  blurb: string;
  price: string;
  period?: string;
  cta: { label: string; href: string };
  featuresLead?: string;
  features: string[];
  highlight?: boolean;
  badge?: string;
};

const PLANS: Plan[] = [
  {
    id: "starter",
    name: "Starter",
    blurb: "For hobby projects and early prototypes.",
    price: "Free",
    cta: { label: "Start free", href: REGISTER_URL },
    featuresLead: "Includes",
    features: [
      "10k exceptions / mo",
      "1 GB ingest / mo",
      "Distributed tracing, logs & metrics",
      "Session replay",
      "Community support",
    ],
  },
  {
    id: "pro",
    name: "Pro",
    blurb: "For small teams shipping to production.",
    price: "$12.99",
    period: "/ mo",
    cta: { label: "Start free", href: REGISTER_URL },
    featuresLead: "Everything in Starter, plus",
    features: [
      "100k exceptions / mo",
      "50 GB ingest / mo",
      "$0.25 / GB beyond",
      "Priority issue triage",
    ],
  },
  {
    id: "premium",
    name: "Premium",
    blurb: "For growing products with real traffic.",
    price: "$24.99",
    period: "/ mo",
    highlight: true,
    badge: "Most popular",
    cta: { label: "Start free", href: REGISTER_URL },
    featuresLead: "Everything in Pro, plus",
    features: [
      "1M exceptions / mo",
      "150 GB ingest / mo",
      "$0.25 / GB beyond",
      "90-day data retention",
    ],
  },
  {
    id: "enterprise",
    name: "Enterprise",
    blurb: "For high volume at the best per-GB rate.",
    price: "$499.99",
    period: "/ mo",
    cta: { label: "Start free", href: REGISTER_URL },
    featuresLead: "Everything in Premium, plus",
    features: [
      "200M exceptions / mo",
      "2 TB ingest / mo",
      "$0.20 / GB beyond",
      "Shared Slack channel",
    ],
  },
  {
    id: "enterprise-plus",
    name: "Enterprise+",
    blurb: "Managed self-hosting with custom SLAs.",
    price: "Custom",
    badge: "Managed self-hosting",
    cta: { label: "Contact us", href: getCalendlyUrl() },
    featuresLead: "Everything in Enterprise, plus",
    features: [
      "Unlimited exceptions",
      "Custom data volume",
      "Dedicated SRE",
      "No data leaves your cloud",
    ],
  },
];

export function PricingPlans() {
  return (
    <div className="grid items-stretch gap-5 md:grid-cols-2 xl:grid-cols-5">
      {PLANS.map((plan) => (
        <div
          key={plan.id}
          className="relative flex flex-col rounded-[20px] px-6 py-7"
          style={
            plan.highlight
              ? {
                  background: "var(--ink-0)",
                  border:
                    "1px solid color-mix(in oklab, var(--a2) 55%, transparent)",
                  boxShadow:
                    "0 2px 6px rgba(10,14,24,0.05), 0 24px 50px -26px color-mix(in oklab, var(--a2) 28%, rgba(10,14,24,0.22))",
                }
              : {
                  background: "var(--ink-0)",
                  border: "1px solid var(--hair-2)",
                  boxShadow:
                    "0 1px 2px rgba(10,14,24,0.04), 0 12px 30px -18px rgba(10,14,24,0.14)",
                }
          }
        >
          {plan.badge ? (
            <span
              className="absolute -top-3 left-1/2 -translate-x-1/2 whitespace-nowrap rounded-full px-3 py-1 text-[11px] font-medium"
              style={
                plan.highlight
                  ? { color: "var(--ink-0)", background: "var(--a2)" }
                  : {
                      color: "var(--ok)",
                      background: "var(--ink-0)",
                      border:
                        "1px solid color-mix(in oklab, var(--ok) 45%, transparent)",
                    }
              }
            >
              {plan.badge}
            </span>
          ) : null}

          <div
            className="text-[17px] font-semibold"
            style={{ color: "var(--fg-0)" }}
          >
            {plan.name}
          </div>
          <p
            className="mt-2 min-h-[38px] text-[13.5px]"
            style={{ color: "var(--fg-2)", lineHeight: 1.45 }}
          >
            {plan.blurb}
          </p>

          <div className="mt-5 flex items-baseline gap-1.5 whitespace-nowrap">
            <span
              className="text-[32px] font-bold leading-none tracking-[-0.02em]"
              style={{ color: "var(--fg-0)" }}
            >
              {plan.price}
            </span>
            {plan.period ? (
              <span className="text-[14px]" style={{ color: "var(--fg-3)" }}>
                {plan.period}
              </span>
            ) : null}
          </div>

          <Link
            href={plan.cta.href}
            className={`btn ${plan.highlight ? "btn-accent" : "btn-ghost"} mt-6 w-full justify-center`}
          >
            {plan.cta.label}
            <ArrowRight className="h-4 w-4" />
          </Link>

          <div className="pt-7">
            {plan.featuresLead ? (
              <div
                className="mb-4 text-[12px] font-medium"
                style={{ color: "var(--fg-2)" }}
              >
                {plan.featuresLead}
              </div>
            ) : null}
            <ul className="flex flex-col gap-3">
              {plan.features.map((f) => (
                <li
                  key={f}
                  className="flex items-start gap-2.5 text-[13.5px]"
                  style={{ color: "var(--fg-1)" }}
                >
                  <Check
                    className="mt-[3px] h-[15px] w-[15px] shrink-0"
                    style={{ color: "var(--ok)" }}
                  />
                  <span style={{ lineHeight: 1.4 }}>{f}</span>
                </li>
              ))}
            </ul>
          </div>
        </div>
      ))}
    </div>
  );
}

type Cell = string | boolean;
type CompareGroup = {
  category: string;
  rows: { label: string; values: Cell[] }[];
};

const PLAN_COLS = ["Starter", "Pro", "Premium", "Enterprise", "Enterprise+"];
const HIGHLIGHT_COL = 2;

const COMPARISON: CompareGroup[] = [
  {
    category: "Pricing",
    rows: [
      {
        label: "Monthly price",
        values: ["Free", "$12.99", "$24.99", "$499.99", "Custom"],
      },
      {
        label: "Data overage rate",
        values: ["—", "$0.25 / GB", "$0.25 / GB", "$0.20 / GB", "Volume rate"],
      },
    ],
  },
  {
    category: "Exceptions",
    rows: [
      {
        label: "Exceptions / month",
        values: ["10k", "100k", "1M", "200M", "Unlimited"],
      },
      {
        label: "Automatic grouping & ranking (SLO)",
        values: [true, true, true, true, true],
      },
      {
        label: "Source-mapped stack traces",
        values: [true, true, true, true, true],
      },
      {
        label: "Archive & resolve workflow (AI)",
        values: [true, true, true, true, true],
      },
    ],
  },
  {
    category: "Data ingest (logs, traces, spans, metrics, replay)",
    rows: [
      {
        label: "Data ingest included / mo",
        values: ["1 GB", "50 GB", "150 GB", "2 TB", "Custom"],
      },
      {
        label: "Beyond included",
        values: [
          "Upgrade to add",
          "$0.25 / GB",
          "$0.25 / GB",
          "$0.20 / GB",
          "Volume rate",
        ],
      },
      { label: "Distributed tracing", values: [true, true, true, true, true] },
      { label: "Session replay", values: [true, true, true, true, true] },
      {
        label: "Logs & custom metrics",
        values: [true, true, true, true, true],
      },
      {
        label: "AI / LLM observability",
        values: [true, true, true, true, true],
      },
    ],
  },
  {
    category: "Platform",
    rows: [
      {
        label: "Projects",
        values: ["3", "10", "Unlimited", "Unlimited", "Unlimited"],
      },
      {
        label: "Team members",
        values: ["3", "Unlimited", "Unlimited", "Unlimited", "Unlimited"],
      },
      {
        label: "Data retention",
        values: ["7 days", "30 days", "90 days", "Custom", "Custom"],
      },
      {
        label: "Alerting & notifications",
        values: [true, true, true, true, true],
      },
    ],
  },
  {
    category: "Support",
    rows: [
      {
        label: "Community & GitHub issues",
        values: [true, true, true, true, true],
      },
      {
        label: "Priority issue triage",
        values: [false, true, true, true, true],
      },
      {
        label: "Shared Slack channel",
        values: [false, false, false, true, true],
      },
      { label: "Dedicated SRE", values: [false, false, false, false, true] },
      { label: "Custom SLAs", values: [false, false, false, false, true] },
      {
        label: "Self-host, data stays in your cloud",
        values: [false, false, false, false, true],
      },
    ],
  },
];

function Value({ value, highlight }: { value: Cell; highlight: boolean }) {
  if (value === true) {
    return (
      <Check
        className="mx-auto h-[17px] w-[17px]"
        style={{ color: highlight ? "var(--a2)" : "var(--ok)" }}
      />
    );
  }
  if (value === false) {
    return (
      <Minus
        className="mx-auto h-[16px] w-[16px]"
        style={{ color: "var(--fg-3)" }}
      />
    );
  }
  return (
    <span
      className="text-[13px]"
      style={{
        color: highlight ? "var(--a2)" : "var(--fg-1)",
        fontWeight: highlight ? 600 : 400,
      }}
    >
      {value}
    </span>
  );
}

const HILITE = "color-mix(in oklab, var(--a2) 8%, transparent)";
const GRID_COLS = "1.7fr repeat(5, 1fr)";

export function PlanComparison() {
  return (
    <div className="overflow-x-auto min-[940px]:overflow-visible">
      <div className="min-w-[880px]">
        {/* Sticky: section title + column headers pin below the navbar (h-16 = 64px) */}
        <div
          className="sticky z-30"
          style={{
            top: "64px",
            background: "var(--ink-0)",
            borderBottom: "1px solid var(--hair-2)",
            boxShadow: "0 10px 18px -14px rgba(10,14,24,0.1)",
          }}
        >
          <h2
            className="px-3 pb-4 pt-6 text-center text-[26px] font-bold tracking-tight"
            style={{ color: "var(--fg-0)" }}
          >
            A closer look at each plan
          </h2>
          <div className="grid" style={{ gridTemplateColumns: GRID_COLS }}>
            <div />
            {PLAN_COLS.map((name, i) => (
              <div
                key={name}
                className="flex items-center justify-center px-3 pb-3 pt-2 text-center text-[14px] font-semibold"
                style={{
                  color: i === HIGHLIGHT_COL ? "var(--a2)" : "var(--fg-0)",
                  background: i === HIGHLIGHT_COL ? HILITE : undefined,
                  borderTopLeftRadius: i === HIGHLIGHT_COL ? 14 : undefined,
                  borderTopRightRadius: i === HIGHLIGHT_COL ? 14 : undefined,
                }}
              >
                {name}
              </div>
            ))}
          </div>
        </div>

        {COMPARISON.map((group, gi) => (
          <div key={group.category}>
            {/* Category */}
            <div
              className="grid"
              style={{
                gridTemplateColumns: GRID_COLS,
                borderTop: gi > 0 ? "1px solid var(--hair-2)" : undefined,
              }}
            >
              <div
                className="px-3 pb-2.5 pt-6 text-[12.5px] font-semibold"
                style={{ color: "var(--fg-2)" }}
              >
                {group.category}
              </div>
              {PLAN_COLS.map((_, ci) => (
                <div
                  key={ci}
                  style={{
                    background: ci === HIGHLIGHT_COL ? HILITE : undefined,
                  }}
                />
              ))}
            </div>
            {/* Rows */}
            {group.rows.map((row, ri) => {
              const lastRow =
                gi === COMPARISON.length - 1 && ri === group.rows.length - 1;
              return (
                <div
                  key={row.label}
                  className="grid"
                  style={{
                    gridTemplateColumns: GRID_COLS,
                    borderTop: "1px solid var(--hair)",
                  }}
                >
                  <div
                    className="flex items-center px-3 py-3.5 text-[13.5px]"
                    style={{ color: "var(--fg-1)" }}
                  >
                    {row.label}
                  </div>
                  {row.values.map((value, ci) => (
                    <div
                      key={ci}
                      className="flex items-center justify-center px-3 py-3.5"
                      style={{
                        background: ci === HIGHLIGHT_COL ? HILITE : undefined,
                        borderBottomLeftRadius:
                          ci === HIGHLIGHT_COL && lastRow ? 14 : undefined,
                        borderBottomRightRadius:
                          ci === HIGHLIGHT_COL && lastRow ? 14 : undefined,
                      }}
                    >
                      <Value value={value} highlight={ci === HIGHLIGHT_COL} />
                    </div>
                  ))}
                </div>
              );
            })}
          </div>
        ))}
      </div>
    </div>
  );
}
