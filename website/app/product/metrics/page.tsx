import Link from "next/link";
import { ArrowRight, BarChart3 } from "lucide-react";

import { Chip } from "@/components/chip";
import { SectionHead } from "@/components/section-head";
import { FeatureRow } from "@/components/feature-row";
import { FaqList } from "@/components/faq-list";
import { FinalCTA } from "@/components/final-cta";
import { AuroraBackground } from "@/components/aurora-background";

export default function MetricsPage() {
  return (
    <main className="relative">
      <section className="hero hero-product relative">
        <AuroraBackground variant="hero" />
        <div className="wrap relative z-10">
          <Chip variant="ok">
            <BarChart3 className="h-3 w-3 inline mr-1" />
            Metrics
          </Chip>
          <h1 className="mt-6">
            Measure what matters, <em>without the bill shock.</em>
          </h1>
          <p className="hero-sub">
            Application metrics via OpenTelemetry, automatic server metrics,
            and flexible widget dashboards, all included, with no per-metric
            billing and no surprise overages.
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

      {/* Application metrics */}
      {/* WHITE BAND: feature sections render on white */}
      <div className="band-light">
        <section className="wrap">
          <FeatureRow
            eyebrow="Application metrics"
            title={
              <>
                Application metrics <em>via OpenTelemetry</em>
              </>
            }
            description="Emit Counter, Gauge, and Histogram metrics through the OpenTelemetry SDK you already have. Traceway ingests OTLP natively, preserves units and dimensional tags, and bills nothing per metric."
            bullets={[
              "OTLP/HTTP and OTLP/gRPC ingestion",
              "Counter / Gauge / Histogram preserved natively",
              "Dimensional tags become facet filters",
              "No per-metric billing",
            ]}
            image={{ src: "/images/metrics-application-dashboard.png", alt: "Application metrics dashboard" }}
          />
        </section>

        {/* Server metrics */}
        <section className="wrap">
          <FeatureRow
            reverse
            eyebrow="Host metrics"
            title="CPU, memory, disk, network. One line to install"
            description={
              <>
                The <Link href="https://github.com/tracewayapp/traceway-otel-agent" style={{ color: "var(--a2)", textDecoration: "underline" }}>Traceway OTel Agent</Link> is a pre-built OpenTelemetry Collector distribution that scrapes host metrics every 60 seconds and ships them to your project over OTLP/HTTP. Install with a single curl, no config file required.
              </>
            }
            bullets={[
              "One-line install on Linux (systemd), macOS (launchd), or Windows",
              "CPU, memory, load, disk, filesystem, network",
              "60-second collection interval via hostmetricsreceiver",
              "Runs alongside your apps, no code changes",
              "Also tails any log files you point it at (opt-in)",
            ]}
            image={{ src: "/images/metrics-server-runtime.png", alt: "Server metrics dashboard" }}
          />
        </section>

        {/* Widget groups */}
        <section className="wrap">
          <FeatureRow
            eyebrow="Widget groups"
            title={
              <>
                Dashboards that match <em>your team&apos;s mental model</em>
              </>
            }
            description="Pick metrics, pick charts, group them into widget pages. No query language required; filters, tag breakdowns, and rollups are all declarative."
            bullets={[
              "Drag-to-add charts",
              "Group widgets by feature, service, or team",
              "Per-metric filters and rollups",
              "Set default dashboards per organization",
            ]}
            image={{ src: "/images/metrics-widget-groups.png", alt: "Widget groups dashboard" }}
          />
        </section>
      </div>

      <FinalCTA
        title={
          <>
            Ship metrics <em>in 5 minutes</em>
          </>
        }
        description="Application + server metrics. Included on every plan. No per-metric billing."
        primary={{
          label: "Read the Metrics docs",
          href: "https://docs.tracewayapp.com",
        }}
      />

      <section className="wrap pt-10 pb-24">
        <div className="max-w-3xl mx-auto">
          <SectionHead align="center" eyebrow="FAQ" title="Questions about metrics" />
          <div className="mt-4">
            <FaqList
              items={[
                {
                  q: "How do I emit an application metric?",
                  a: (
                    <>
                      <p>
                        Point any OpenTelemetry SDK at{" "}
                        <code>/api/otel/v1/metrics</code>. OTLP/HTTP and
                        OTLP/gRPC are both supported natively. Counter, Gauge,
                        and Histogram metric types are ingested as-is, and
                        dimensional tags become facet filters in the dashboard.
                      </p>
                      <p>
                        If you&apos;re instrumenting from scratch, the
                        OpenTelemetry metrics SDK is the recommended path for
                        every language we support.
                      </p>
                    </>
                  ),
                },
                {
                  q: "What host metrics are collected automatically?",
                  a: (
                    <>
                      <p>
                        The{" "}
                        <Link href="https://github.com/tracewayapp/traceway-otel-agent" style={{ color: "var(--a2)", textDecoration: "underline" }}>
                          Traceway OTel Agent
                        </Link>{" "}
                        is a pre-built OpenTelemetry Collector distribution that
                        you install on the host with a single curl. It scrapes
                        CPU, memory, load, disk, filesystem, and network
                        metrics via the upstream{" "}
                        <code>hostmetricsreceiver</code> every 60 seconds and
                        ships them to your Traceway project over OTLP/HTTP.
                        Per-process metrics are opt-in via{" "}
                        <code>TRACEWAY_PROCESS_NAMES</code>.
                      </p>
                      <p>
                        No config file to write. You set{" "}
                        <code>TRACEWAY_TOKEN</code> and the installer wires up
                        systemd/launchd/Windows service for you. In-process Go
                        runtime metrics (goroutines, heap objects, GC) are
                        emitted separately by the Go client SDK if you use it.
                      </p>
                    </>
                  ),
                },
                {
                  q: "Do custom metrics count toward my event limit?",
                  a: "No. Metrics are included at no additional event cost. Only issues, HTTP requests, and background tasks count toward your event limit. This means you can emit thousands of custom metrics without worrying about billing.",
                },
                {
                  q: "Can I query metrics by tag or dimension?",
                  a: "Yes. Every tag becomes a facet you can filter on; widget groups let you build per-dimension chart panels. For example, a `plan` tag on a signups metric lets you chart signups broken down by plan, region, or tenant.",
                },
              ]}
            />
          </div>
        </div>
      </section>
    </main>
  );
}
