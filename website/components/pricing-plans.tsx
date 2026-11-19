import Link from "next/link";
import { ArrowRight, Check } from "lucide-react";

import { getCalendlyUrl } from "@/lib/calendly";

const REGISTER_URL = "https://cloud.tracewayapp.com/register";

type Plan = {
  id: string;
  name: string;
  blurb: string;
  price: string;
  period?: string;
  cta: { label: string; href: string };
  features: string[];
  highlight?: boolean;
};

const PLANS: Plan[] = [
  {
    id: "starter",
    name: "Starter",
    blurb: "For side projects and trying Traceway out.",
    price: "Free",
    cta: { label: "Start free", href: REGISTER_URL },
    features: [
      "10k exceptions / mo",
      "1 GB data ingest / mo",
      "3 projects",
      "3 team members",
      "5 monitors",
    ],
  },
  {
    id: "premium",
    name: "Premium",
    blurb: "For teams running in production.",
    price: "$24.99",
    period: "/ mo",
    highlight: true,
    cta: { label: "Get started", href: REGISTER_URL },
    features: [
      "1M exceptions / mo",
      "150 GB data ingest / mo",
      "Unlimited projects",
      "Unlimited team members",
      "200 monitors",
      "Optional overage at $0.25 / GB",
    ],
  },
  {
    id: "enterprise",
    name: "Enterprise",
    blurb: "For high volume and custom needs.",
    price: "Custom",
    cta: { label: "Contact us", href: getCalendlyUrl() },
    features: [
      "Exceptions and data sized to your volume",
      "Custom project, member and monitor limits",
      "Monthly or annual billing",
      "Shared Slack channel with our team",
    ],
  },
];

export function PricingPlans() {
  return (
    <div className="mx-auto grid max-w-[1040px] items-stretch gap-5 md:grid-cols-3">
      {PLANS.map((plan) => (
        <div
          key={plan.id}
          className="flex flex-col rounded-[20px] px-6 py-7"
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
          <div
            className="text-[17px] font-semibold"
            style={{ color: "var(--fg-0)" }}
          >
            {plan.name}
          </div>
          <p
            className="mt-2 text-[13.5px]"
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

          <ul className="flex flex-col gap-3 pt-7">
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
      ))}
    </div>
  );
}
