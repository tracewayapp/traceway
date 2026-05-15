import Link from "next/link";
import Image from "next/image";
import { ArrowRight, Activity, ChartGantt, Gauge } from "lucide-react";

import { Chip } from "@/components/chip";
import { SectionHead } from "@/components/section-head";
import { FeatureRow } from "@/components/feature-row";
import { FaqList } from "@/components/faq-list";
import { FinalCTA } from "@/components/final-cta";
import { AuroraBackground } from "@/components/aurora-background";

export default function PerformancePage() {
  return (
    <main className="relative">
      <section className="hero hero-product gridbg relative">
        <AuroraBackground variant="hero" />
        <div className="wrap relative z-10">
          <Chip>
            <Activity className="h-3 w-3 inline mr-1" />
            Performance
          </Chip>
          <h1 className="mt-6">
            Understand exactly <em>where time is spent.</em>
          </h1>
          <p className="hero-sub">
            P50/P95/P99 percentiles for every endpoint, span waterfall traces,
            and per-endpoint drill-downs. Know exactly what&apos;s slow and why.
          </p>
          <div className="hero-cta-row">
            <Link href="https://docs.tracewayapp.com" className="btn btn-accent">
              Get Started <ArrowRight className="h-4 w-4" />
            </Link>
            <Link href="https://cloud.tracewayapp.com/register" className="btn btn-ghost">
              Try Traceway Cloud
            </Link>
          </div>
        </div>
      </section>

      {/* Impact Score — scoring and ranking logic */}
      <section className="wrap py-20">
        <SectionHead
          eyebrow="Impact Score"
          title={
            <>
              One score. Five signals. <em>Zero guesswork.</em>
            </>
          }
          description="The Impact Score combines five service-level indicators into one automatic priority for every endpoint. It takes the max across all five — if any single signal is bad, the endpoint surfaces immediately."
        />
        <div className="mt-10 max-w-5xl mx-auto">
          <Image
            src="/images/performance-endpoints-impact-table.png"
            alt="Endpoints sorted by Impact Score"
            width={1600}
            height={1000}
            className="w-full h-auto rounded-[12px]"
            style={{ border: "1px solid var(--hair)" }}
          />
        </div>
      </section>

      {/* Drill into any request — absorbed from home */}
      <section className="wrap">
        <FeatureRow
          eyebrow="Endpoint introspection"
          title={
            <>
              Drill into <em>any request</em>
            </>
          }
          description="Request/response details, waterfall traces, and custom context tags. Understand the exact state of your application for every single trace — not just aggregates."
          bullets={[
            "Detailed request/response payloads",
            "Waterfall trace view with span attributes",
            "Custom context and tag facets",
            "Filter by status, latency, or tag",
          ]}
          image={{ src: "/images/performance-endpoint-drilldown.png", alt: "Endpoint introspection" }}
        />
      </section>

      {/* Endpoint analytics */}
      <section className="wrap">
        <FeatureRow
          reverse
          eyebrow="Percentiles"
          title="Endpoint analytics at a glance"
          description="See P50, P95, and P99 percentiles for every endpoint in your application. Quickly identify which routes are fast and which need attention, with throughput and error-rate breakdowns."
          bullets={[
            "P50 / P95 / P99 latency percentiles",
            "Throughput + error rate per route",
            "Historical trend comparison",
            "Per-endpoint slow threshold override",
          ]}
          image={{ src: "/images/performance-percentiles-overview.png", alt: "Endpoint analytics" }}
        />
      </section>

      {/* Waterfall */}
      <section className="wrap">
        <FeatureRow
          eyebrow="Waterfall"
          title="Span waterfall view"
          description="Break down every request into its component operations. See exactly which database query, external API call, or middleware is adding latency, with precise timing for each span."
          bullets={[
            "Operation-level timing breakdown",
            "Visual waterfall timeline",
            "Pinpoint bottlenecks instantly",
            "Expand spans to see logs + attributes",
          ]}
          image={{ src: "/images/traces-spans-waterfall.png", alt: "Span waterfall view", width: 1600, height: 500 }}
        />
      </section>

      {/* Apdex + slow threshold */}
      <section className="wrap py-10">
        <SectionHead
          eyebrow="Apdex-aware"
          title={
            <>
              Know what counts as <em>slow</em>
            </>
          }
          description="Set a slow-endpoint threshold globally or per route. Traceway uses it as the apdex anchor — a deliberate knob so /api/health and /api/checkout can be judged on their own terms."
        />
        <div
          className="mt-8 max-w-3xl mx-auto rounded-[12px] p-8"
          style={{
            background: "linear-gradient(180deg, var(--ink-3), var(--ink-2))",
            border: "1px solid var(--hair)",
          }}
        >
          <div className="grid grid-cols-3 gap-6 text-center">
            <div>
              <div
                className="text-[32px] font-semibold"
                style={{ color: "var(--ok)", fontFamily: "var(--font-display)" }}
              >
                ≤ 750ms
              </div>
              <div
                className="mt-2 text-[11px] uppercase tracking-[0.1em]"
                style={{ color: "var(--fg-2)", fontFamily: "var(--font-mono)" }}
              >
                Good
              </div>
            </div>
            <div>
              <div
                className="text-[32px] font-semibold"
                style={{ color: "var(--a4)", fontFamily: "var(--font-display)" }}
              >
                ≤ 1500ms
              </div>
              <div
                className="mt-2 text-[11px] uppercase tracking-[0.1em]"
                style={{ color: "var(--fg-2)", fontFamily: "var(--font-mono)" }}
              >
                Tolerable
              </div>
            </div>
            <div>
              <div
                className="text-[32px] font-semibold"
                style={{ color: "var(--crit)", fontFamily: "var(--font-display)" }}
              >
                &gt; 1500ms
              </div>
              <div
                className="mt-2 text-[11px] uppercase tracking-[0.1em]"
                style={{ color: "var(--fg-2)", fontFamily: "var(--font-mono)" }}
              >
                Bad or 5xx
              </div>
            </div>
          </div>
          <div
            className="mt-6 pt-6 text-center text-sm flex items-center justify-center gap-3"
            style={{ borderTop: "1px solid var(--hair)", color: "var(--fg-2)" }}
          >
            <Gauge className="h-4 w-4" style={{ color: "var(--a2)" }} />
            Override per endpoint via the slow threshold UI
          </div>
        </div>
      </section>

      <FinalCTA
        title={
          <>
            Measure. <em>Find.</em> Fix.
          </>
        }
        description="Performance dashboards included in every plan."
        primary={{ label: "Get Started", href: "https://docs.tracewayapp.com" }}
        secondary={{
          label: "Try Live Demo",
          href: "https://cloud.tracewayapp.com/login?email=demo@tracewayapp.com&password=demoaccount!",
        }}
      />

      <section className="wrap pt-10 pb-24">
        <div className="max-w-3xl mx-auto">
          <SectionHead align="center" eyebrow="FAQ" title="Questions about performance" />
          <div className="mt-4">
            <FaqList
              items={[
                {
                  q: "What is the Impact Score?",
                  a: (
                    <>
                      <p>
                        The Impact Score is Traceway&apos;s automatic prioritization
                        system. It combines five service-level indicators — an
                        inverted apdex variant, error rate floor, P99 latency
                        floor, client error floor, and volume error floor — into
                        a single 0-100 score for every endpoint.
                      </p>
                      <p>
                        It takes the max across all five, so if any single
                        signal is degraded that endpoint surfaces immediately.
                        You can adjust the slow endpoint threshold per endpoint
                        to tune the apdex calculation for routes with different
                        latency profiles.
                      </p>
                      <p>
                        For incident management, notification rules fire when
                        the Impact Score or error rate crosses a threshold, and
                        route alerts to Slack, GitHub Issues, email, Pushover, or custom
                        webhooks.
                      </p>
                    </>
                  ),
                },
                {
                  q: "What percentiles does Traceway track?",
                  a: "Traceway calculates P50 (median), P95, and P99 latency percentiles for every endpoint. This gives you a clear picture of both typical and worst-case response times for your application.",
                },
                {
                  q: "How does the waterfall view work?",
                  a: "The waterfall view shows every span (database query, external API call, middleware, etc.) within a single request as a timeline. You can see how long each operation took and where they overlap, making it easy to identify the slowest part of any request.",
                },
                {
                  q: "How are the apdex thresholds computed?",
                  a: "Traceway uses a slow-endpoint threshold as the apdex anchor. Requests under 750ms are Good, under 1500ms are Tolerable, and over 1500ms (or returning 5xx) are Bad. You can override the threshold globally or per-endpoint to account for routes that have different latency profiles — /api/health and /api/checkout rightly judged on their own terms.",
                },
                {
                  q: "Where do server metrics live now?",
                  a: (
                    <>
                      <p>
                        Server metrics (CPU, memory, goroutines, GC) moved to
                        the dedicated{" "}
                        <Link href="/product/metrics" style={{ color: "var(--a2)", textDecoration: "underline" }}>
                          Metrics
                        </Link>{" "}
                        page. Performance stays focused on latency, percentiles,
                        and waterfall span analysis.
                      </p>
                    </>
                  ),
                },
              ]}
            />
          </div>
        </div>
      </section>
    </main>
  );
}
