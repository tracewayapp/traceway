import Link from "next/link";
import Image from "next/image";
import {
  ArrowRight,
  Video,
  ScrollText,
  Network,
  BarChart3,
  Workflow,
  Bug,
  Github,
} from "lucide-react";

import { Eyebrow } from "@/components/eyebrow";
import { DiscordIcon } from "@/components/discord-icon";
import { FinalCTA } from "@/components/final-cta";
import { Terminal } from "@/components/terminal";
import { StatsStrip } from "@/components/stats-strip";
import { HeroEmailCTA } from "@/components/hero-email-cta";
import { getCalendlyUrl } from "@/lib/calendly";
import { GITHUB_URL, DISCORD_URL } from "@/lib/links";

const PILLARS = [
  {
    icon: ScrollText,
    title: "Logs",
    description: "Structured, trace-linked, sub-second search",
    href: "/product/logs",
  },
  {
    icon: Network,
    title: "Traces",
    description: "End-to-end span waterfalls across every service",
    href: "/product/traces",
  },
  {
    icon: BarChart3,
    title: "Metrics",
    description: "Host, runtime, and custom metrics on any chart",
    href: "/product/metrics",
  },
  {
    icon: Video,
    title: "Session replay",
    description: "Web DOM capture, attached to exceptions",
    href: "/product/session-replay",
  },
  {
    icon: Bug,
    title: "Exceptions",
    description: "Grouped, normalized, paired with replay",
    href: "/product/stack-traces",
  },
  {
    icon: Workflow,
    title: "AI tracing",
    description: "LLM cost, tokens, conversations",
    href: "/product/ai-tracing",
  },
];

export default function Home() {
  return (
    <main className="relative">
      {/* HERO: centered chip, title, subhead, email form */}
      <section className="hero">
        <div className="wrap">
          <div className="text-center max-w-4xl mx-auto flex flex-col items-center">
            <Image
              src="/images/frameworks/otel.png"
              alt="OpenTelemetry"
              width={56}
              height={56}
              className="mb-5"
            />
            <h1 className="mt-6">
              The open-source APM <em>built on OpenTelemetry.</em>
            </h1>
            <p className="hero-sub text-pretty">
              Complete observability. Logs, traces, metrics, session replay,
              and exceptions, all connected.
            </p>

            <div className="mt-10 w-full">
              <HeroEmailCTA />
            </div>
          </div>
        </div>
      </section>

      {/* PRODUCT: the dashboard itself, then the six pillars as a plain list */}
      <section className="pt-10 pb-24">
        <div className="wrap">
          <Image
            src="/images/home-hero-overview.png"
            alt="Traceway dashboard: endpoints overview with impact scoring"
            width={2336}
            height={1532}
            priority
            className="w-full h-auto rounded-[12px] border border-hair-2 bg-ink-1"
          />

          <div className="mt-16 grid gap-x-12 gap-y-10 sm:grid-cols-2 lg:grid-cols-3">
            {PILLARS.map((pillar) => (
              <Link key={pillar.href} href={pillar.href} className="group">
                <div className="flex items-center gap-2.5">
                  <pillar.icon className="size-[18px] text-fg-2" aria-hidden />
                  <h3 className="text-base font-semibold text-fg-0">
                    {pillar.title}
                  </h3>
                  <ArrowRight
                    className="size-3.5 text-fg-3 opacity-0 -translate-x-1 transition group-hover:opacity-100 group-hover:translate-x-0"
                    aria-hidden
                  />
                </div>
                <p className="muted mt-2">{pillar.description}</p>
              </Link>
            ))}
          </div>
        </div>
      </section>

      {/* WHITE BAND: deploy, detect/resolve, cost render on white */}
      <div className="band-light">
        {/* DEPLOY: stats strip + terminal */}
        <section className="py-24">
          <div className="wrap grid gap-14 md:grid-cols-[10fr_11fr] items-center">
            <div>
              <Eyebrow>Your data. Your metal.</Eyebrow>
              <h2 className="mt-4">
                Self-host in <em>90 seconds flat.</em>
              </h2>
              <p className="muted mt-4 max-w-[460px] text-pretty">
                MIT licensed, full feature parity with Cloud. Point an OTLP
                exporter at it and you&apos;re in business.
              </p>
              <StatsStrip
                stats={[
                  { num: "<em>0s</em>", label: "Config required" },
                  { num: "100%", label: "Feature parity" },
                  { num: "MIT", label: "License" },
                ]}
              />
            </div>
            <Terminal
              title="bash · traceway.sh · 80×24"
              lines={[
                {
                  ln: "1",
                  type: "tx",
                  content: (
                    <>
                      <span className="cmd">$</span> git clone
                      github.com/tracewayapp/traceway
                    </>
                  ),
                },
                {
                  ln: "2",
                  type: "tx",
                  content: (
                    <>
                      <span className="cmd">$</span> cd traceway &amp;&amp; docker
                      compose up -d
                    </>
                  ),
                },
                { ln: "3", type: "mute", content: "# pulling images…" },
                {
                  ln: "4",
                  type: "mute",
                  content: "# starting clickhouse · postgres · collector",
                },
                {
                  ln: "5",
                  type: "ok",
                  content: "# ✓ dashboard at http://localhost:3000",
                },
                {
                  ln: "6",
                  type: "tx",
                  content: (
                    <>
                      <span className="cmd">$</span>
                    </>
                  ),
                },
              ]}
              showCursor
            />
          </div>
        </section>

        {/* DETECT → RESOLVE: two quiet steps, no glow tracks */}
        <section className="py-24">
          <div className="wrap">
            <Eyebrow>Why it matters</Eyebrow>
            <h2 className="mt-4 max-w-[24ch]">
              Customers don&apos;t complain, they quit.{" "}
              <em>We stop the bleeding.</em>
            </h2>
            <p className="muted mt-4 max-w-[640px] text-pretty">
              Traceway catches the error, the session replay, and the exact
              failing span before your users close the tab.
            </p>

            <div className="mt-14 grid gap-12 md:grid-cols-2">
              <div className="border-t border-hair pt-8">
                <p className="font-mono text-[0.6875rem] uppercase tracking-[0.14em] text-fg-3">
                  01 · Detect
                </p>
                <h3 className="mt-3">Surface what actually matters.</h3>
                <p className="muted mt-3 max-w-[440px] text-pretty">
                  Impact Score ranks every endpoint by five service-level
                  signals and routes alerts to Slack, GitHub, or webhook. No
                  false-positive fatigue.
                </p>
              </div>
              <div className="border-t border-hair pt-8">
                <p className="font-mono text-[0.6875rem] uppercase tracking-[0.14em] text-fg-3">
                  02 · Resolve
                </p>
                <h3 className="mt-3">Walk the full trace. Fix. Ship.</h3>
                <p className="muted mt-3 max-w-[440px] text-pretty">
                  Click an exception to see the session replay, the
                  cross-service trace, and the source-mapped stack.
                  Context-switching is the bug.
                </p>
              </div>
            </div>
          </div>
        </section>

        {/* COST: the closing argument before community */}
        <section className="py-24">
          <div className="wrap grid gap-14 md:grid-cols-[10fr_11fr] items-center">
            <div>
              <Eyebrow>Pricing that doesn&apos;t lie to you</Eyebrow>
              <h2 className="mt-4">
                A fraction of the cost. <em>None of the asterisks.</em>
              </h2>
              <p className="muted mt-4 max-w-[460px] text-pretty">
                ClickHouse columnar storage compresses 1M daily events to ~2
                GB/month. Fixed monthly tiers, no per-event gouging.
              </p>
              <div className="mt-6 flex flex-wrap gap-3">
                <Link href="/cloud" className="btn btn-accent">
                  See pricing
                  <ArrowRight className="h-4 w-4" />
                </Link>
                <Link
                  href="https://docs.tracewayapp.com"
                  className="btn btn-ghost"
                >
                  Self-host for free
                </Link>
              </div>
            </div>
            <Image
              src="/images/performance-endpoints-impact-table.png"
              alt="Traceway endpoints ranked by impact score"
              width={2470}
              height={1548}
              className="w-full h-auto rounded-[12px] border border-hair bg-ink-1"
            />
          </div>
        </section>
      </div>

      {/* COMMUNITY: built in the open */}
      <section className="py-24">
        <div className="wrap">
          <Eyebrow>Community</Eyebrow>
          <h2 className="mt-4">Built in the open.</h2>
          <p className="muted mt-4 max-w-[640px] text-pretty">
            Traceway is MIT licensed and developed in public. Star the repo,
            file issues, and help shape the roadmap.
          </p>
          <div className="mt-6 flex flex-wrap gap-3">
            <Link
              href={GITHUB_URL}
              target="_blank"
              rel="noopener noreferrer"
              className="btn btn-ghost"
            >
              <Github className="h-4 w-4" />
              Star on GitHub
            </Link>
            <Link
              href={DISCORD_URL}
              target="_blank"
              rel="noopener noreferrer"
              className="btn btn-ghost"
            >
              <DiscordIcon className="h-4 w-4" />
              Join the Discord
            </Link>
          </div>
        </div>
      </section>

      {/* Final CTA */}
      <FinalCTA
        title={
          <>
            Detect. Replay. <em>Resolve.</em>
          </>
        }
        description="Start for free. Self-host whenever you want. Book a demo if you'd like a walkthrough."
        primary={{
          label: "Start for free",
          href: "https://cloud.tracewayapp.com/register",
        }}
        secondary={{
          label: "Book a demo",
          href: getCalendlyUrl(),
        }}
      />
    </main>
  );
}
