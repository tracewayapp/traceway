import Link from "next/link";
import { ArrowRight, ScrollText, Network, Boxes } from "lucide-react";

import { Chip } from "@/components/chip";
import { SectionHead } from "@/components/section-head";
import { FeatureRow } from "@/components/feature-row";
import { BentoGrid, BentoCell } from "@/components/bento-grid";
import { FaqList } from "@/components/faq-list";
import { FinalCTA } from "@/components/final-cta";
import { AuroraBackground } from "@/components/aurora-background";

export default function LogsPage() {
  return (
    <main className="relative">
      <section className="hero hero-product relative">
        <AuroraBackground variant="hero" />
        <div className="wrap relative z-10">
          <Chip>
            <ScrollText className="h-3 w-3 inline mr-1" />
            Logs
          </Chip>
          <h1 className="mt-6">
            Every log, <em>with its full trace attached.</em>
          </h1>
          <p className="hero-sub">
            Stop jumping between a log viewer and a trace viewer. Search by
            severity, service, or attribute, then open the exact request that
            produced any log line in one click.
          </p>
          <div className="hero-cta-row">
            <Link href="https://docs.tracewayapp.com/learn/logs" className="btn btn-accent">
              Get Started <ArrowRight className="h-4 w-4" />
            </Link>
            <Link href="https://cloud.tracewayapp.com/register" className="btn btn-ghost">
              Try Traceway Cloud
            </Link>
          </div>
        </div>
      </section>

      {/* Search across every log */}
      {/* WHITE BAND: feature sections render on white */}
      <div className="band-light">
        <section className="wrap">
          <FeatureRow
            eyebrow="Search"
            title={
              <>
                Search across <em>every log</em>
              </>
            }
            description="Filter by severity, service, or trace. Full-text search over log bodies is powered by a token index, and any resource, scope, or log attribute can be queried for exact matches."
            bullets={[
              "Body search backed by token indexes",
              "Six severity levels: TRACE, DEBUG, INFO, WARN, ERROR, FATAL",
              "Attribute filters on resource, scope, and log fields",
              "Time-range facet navigation",
            ]}
            image={{ src: "/images/logs-search-and-detail.png", alt: "Logs search and detail view", width: 1600, height: 1000 }}
          />
        </section>

        {/* 2-card bento */}
        <section className="wrap py-10">
          <SectionHead
            eyebrow="Built for correlation"
            title={
              <>
                Linked to traces. <em>Native to OpenTelemetry.</em>
              </>
            }
          />
          <BentoGrid>
            <BentoCell
              size="wide"
              icon={Network}
              title="Linked to every trace"
              iconColor="var(--a1)"
            >
              <p>
                Every log carries the trace and span ID of the request that
                emitted it. Open any endpoint and see the exact log lines tied to
                that invocation, or follow a distributed trace to see logs from
                every service it touched.
              </p>
            </BentoCell>
            <BentoCell
              size="tall"
              icon={Boxes}
              title="OpenTelemetry-native"
              iconColor="var(--ok)"
            >
              <p>
                Send logs from any OTel SDK: Node.js, Python, Go, Java, .NET,
                PHP. No vendor client needed. OTLP/HTTP supports Protobuf and
                JSON, with a 30-day TTL.
              </p>
            </BentoCell>
          </BentoGrid>
        </section>
      </div>

      <FinalCTA
        title={
          <>
            One log. <em>One click to its trace.</em>
          </>
        }
        description="Logs are included on every plan, from Starter to Enterprise."
        primary={{ label: "Read the Logs docs", href: "https://docs.tracewayapp.com/learn/logs" }}
      />

      <section className="wrap pt-10 pb-24">
        <div className="max-w-3xl mx-auto">
          <SectionHead align="center" eyebrow="FAQ" title="Questions about logs" />
          <div className="mt-4">
            <FaqList
              items={[
                {
                  q: "How do I send logs to Traceway?",
                  a: (
                    <>
                      <p>
                        Point any OpenTelemetry Logs SDK at{" "}
                        <code>/api/otel/v1/logs</code> with an{" "}
                        <code>Authorization: Bearer &lt;project_token&gt;</code>{" "}
                        header. Protobuf and JSON are both supported.
                      </p>
                      <p>
                        If you already run an OTel Collector, route its{" "}
                        <code>logs</code> pipeline to the same endpoint.
                      </p>
                    </>
                  ),
                },
                {
                  q: "Are logs linked to my traces automatically?",
                  a: (
                    <>
                      <p>
                        Yes. Every log carries the OTel <code>trace_id</code> and{" "}
                        <code>span_id</code> from the active context at emission
                        time. That means logs emitted inside a request handler,
                        background job, or child span are automatically
                        associated with the corresponding trace, with no extra
                        plumbing required.
                      </p>
                    </>
                  ),
                },
                {
                  q: "How long are logs retained?",
                  a: "30 days by default. ClickHouse-backed storage with daily partitioning and TTL, so retention is enforced automatically without manual cleanup.",
                },
                {
                  q: "Do logs count toward my event limit?",
                  a: "Logs have their own ingestion tier that scales with your plan. See the Cloud pricing page for current limits. Self-hosting is unlimited.",
                },
              ]}
            />
          </div>
        </div>
      </section>
    </main>
  );
}
