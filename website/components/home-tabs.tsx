"use client";

import { useState } from "react";
import Link from "next/link";
import { Server, Monitor, Network, Workflow, Radio, ArrowRight } from "lucide-react";
import type { LucideIcon } from "lucide-react";

type Tab = {
  id: string;
  label: string;
  icon: LucideIcon;
  color: string;
  heading: string;
  description: string;
  bullets: string[];
  href: string;
};

const tabs: Tab[] = [
  {
    id: "backend",
    label: "Backend",
    icon: Server,
    color: "var(--a2)",
    heading: "Trace every request. Rank every endpoint.",
    description:
      "Traceway captures detailed span waterfall traces for every backend request, monitors scheduled tasks and background jobs, and ranks endpoints by real user impact.",
    bullets: [
      "Full request/response span traces",
      "Scheduled task and background job monitoring",
      "Automatic Impact Score ranking",
      "OpenTelemetry-native ingestion",
      "Works with serverless (Lambda, Cloud Functions, Workers)",
    ],
    href: "/product/performance",
  },
  {
    id: "frontend",
    label: "Frontend",
    icon: Monitor,
    color: "var(--a1)",
    heading: "Replay the moment before every error.",
    description:
      "See exactly what users did before an exception. Session replays are attached to errors automatically. Source map resolution turns minified errors into readable, actionable traces.",
    bullets: [
      "Session replay with pre-error capture",
      "Automatic source map stack trace resolution",
      "Click, scroll, and navigation tracking",
      "Linked directly to backend traces",
    ],
    href: "/product/session-replay",
  },
  {
    id: "microservice",
    label: "Microservice",
    icon: Network,
    color: "var(--ok)",
    heading: "Follow a request across every service.",
    description:
      "Distributed tracing connects frontend sessions to backend errors across your entire microservice topology. When a backend service throws an exception, you see the user's session replay, the full cross-service trace, and the exact span that failed.",
    bullets: [
      "Cross-service distributed trace propagation",
      "Frontend sessions linked to backend errors",
      "Full trace context across service boundaries",
      "Exception pinpointing across services",
    ],
    href: "/product/traces",
  },
  {
    id: "ai-agents",
    label: "AI Agents",
    icon: Workflow,
    color: "var(--a3)",
    heading: "Track every AI call, its cost, and its conversation.",
    description:
      "Monitor LLM costs, token usage, and latency across every provider. See the full prompt and completion for every call, with per-agent and per-model breakdowns.",
    bullets: [
      "Per-call cost and token tracking",
      "Conversation replay with chat view",
      "P50/P95 latency per agent and model",
      "Works with OpenRouter and any OTel provider",
    ],
    href: "/product/ai-tracing",
  },
  {
    id: "iot",
    label: "IoT",
    icon: Radio,
    color: "var(--a4)",
    heading: "Monitor fleets of devices at scale.",
    description:
      "Traceway's OpenTelemetry-native ingestion and ClickHouse columnar storage handle high-volume telemetry from IoT devices efficiently. Enterprise pricing supports large device fleets with predictable, fixed costs. No per-event billing surprises.",
    bullets: [
      "High-volume ingestion via OTLP/HTTP",
      "ClickHouse compression keeps storage costs low",
      "Enterprise pricing for large device fleets",
      "Custom metrics for device health and telemetry",
    ],
    href: "/cloud",
  },
];

export function HomeTabs() {
  const [active, setActive] = useState(0);
  const tab = tabs[active];
  const Icon = tab.icon;

  return (
    <div>
      <div className="flex items-center justify-center mb-8">
        <div
          className="inline-flex items-center gap-1 p-1 rounded-lg"
          style={{
            background: "color-mix(in oklab, var(--ink-3) 80%, transparent)",
            border: "1px solid var(--hair)",
          }}
        >
          {tabs.map((t, i) => (
            <button
              key={t.id}
              onClick={() => setActive(i)}
              className="px-4 py-2 rounded-md text-[13px] font-medium transition-all"
              style={{
                fontFamily: "var(--font-display)",
                background: active === i ? "var(--ink-0)" : "transparent",
                color: active === i ? "var(--fg-0)" : "var(--fg-2)",
                boxShadow: active === i ? "0 1px 2px rgba(0,0,0,0.1)" : "none",
              }}
            >
              {t.label}
            </button>
          ))}
        </div>
      </div>

      <div
        className="rounded-[14px] p-8 md:p-10"
        style={{
          background:
            "linear-gradient(180deg, color-mix(in oklab, var(--ink-3) 30%, transparent), color-mix(in oklab, var(--ink-2) 20%, transparent))",
          border: "1px solid var(--hair)",
        }}
      >
        <div className="flex flex-col md:flex-row items-start gap-8">
          <div className="flex-1 space-y-5">
            <div
              className="w-12 h-12 rounded-[10px] flex items-center justify-center"
              style={{
                background: `color-mix(in oklab, ${tab.color} 18%, transparent)`,
                border: `1px solid color-mix(in oklab, ${tab.color} 40%, transparent)`,
                color: tab.color,
              }}
            >
              <Icon className="w-5 h-5" />
            </div>
            <h3 className="text-xl md:text-2xl font-semibold tracking-tight" style={{ color: "var(--fg-0)" }}>
              {tab.heading}
            </h3>
            <p className="leading-relaxed" style={{ color: "var(--fg-1)" }}>
              {tab.description}
            </p>
            <ul className="space-y-3 pt-1">
              {tab.bullets.map((b) => (
                <li
                  key={b}
                  className="flex items-center gap-3 text-[15px]"
                  style={{ color: "var(--fg-1)" }}
                >
                  <div
                    className="w-1.5 h-1.5 rounded-full"
                    style={{ background: tab.color }}
                  />
                  {b}
                </li>
              ))}
            </ul>
            <Link
              href={tab.href}
              className="inline-flex items-center gap-1 text-sm font-medium transition-colors pt-2 hover:opacity-80"
              style={{ color: tab.color }}
            >
              Learn more <ArrowRight className="w-3.5 h-3.5" />
            </Link>
          </div>
        </div>
      </div>
    </div>
  );
}
