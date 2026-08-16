import Link from "next/link";
import Image from "next/image";
import { ArrowRight, ScrollText, Network, Boxes } from "lucide-react";

import { Chip } from "@/components/chip";
import { SectionHead } from "@/components/section-head";
import { FeatureRow } from "@/components/feature-row";
import { BentoGrid, BentoCell } from "@/components/bento-grid";
import { FaqList } from "@/components/faq-list";
import { FinalCTA } from "@/components/final-cta";
import { AuroraBackground } from "@/components/aurora-background";

function Shot({
  src,
  alt,
  width,
  height,
}: {
  src: string;
  alt: string;
  width: number;
  height: number;
}) {
  return (
    <div
      className="rounded-[12px] overflow-hidden bg-white"
      style={{ border: "1px solid var(--hair)", boxShadow: "0 16px 48px rgba(15, 23, 42, 0.14)" }}
    >
      <Image src={src} alt={alt} width={width} height={height} className="w-full h-auto block" />
    </div>
  );
}

export default function LogsPage() {
  return (
    <main className="relative">
      <section className="hero hero-product relative">
        <AuroraBackground variant="hero" />
        <div className="wrap relative z-10">
          <div className="lg:flex lg:items-center lg:gap-14">
            <div className="shrink-0 lg:w-[44%]">
              <Chip>
                <ScrollText className="h-3 w-3 inline mr-1" />
                Logs
              </Chip>
              <h1 className="mt-6">
                Every log, <em>with its full trace attached.</em>
              </h1>
              <p className="hero-sub">
                Stop jumping between a log viewer and a trace viewer. Search by
                severity, service, or attribute, tail new logs live as they arrive,
                and open the exact request that produced any log line in one click.
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
            <div className="mt-12 lg:mt-0 lg:flex-1">
              <Image
                src="/images/logs-search-and-detail.png"
                alt="Logs search with attribute filters applied"
                width={1600}
                height={900}
                priority
                className="w-full h-auto rounded-[12px]"
                style={{
                  border: "1px solid var(--hair-2)",
                  boxShadow: "0 24px 80px rgba(0, 0, 0, 0.45)",
                }}
              />
            </div>
          </div>
        </div>
      </section>

      {/* WHITE BAND: feature sections render on white */}
      <div className="band-light">
        {/* Filtering close-ups */}
        <section className="wrap py-20">
          <SectionHead
            eyebrow="Filters"
            title={
              <>
                Include what matters. <em>Exclude the noise.</em>
              </>
            }
            description="Every filter is a pill you can see, edit, and remove. Flip the operator to not-equals to mute a chatty logger, and set a severity threshold to keep a level and everything above it."
          />
          <div className="max-w-5xl mx-auto">
            <div className="md:flex md:items-start">
              <div className="relative z-0 md:mt-24 md:w-[56%]">
                <Shot
                  src="/images/logs-severity-filter.png"
                  alt="Severity threshold dropdown"
                  width={1148}
                  height={748}
                />
              </div>
              <div className="relative z-10 mt-6 md:-ml-[14%] md:mt-0 md:w-[58%]">
                <Shot
                  src="/images/logs-filter-dialog-negative.png"
                  alt="Attribute filter dialog with the Not equals operator selected"
                  width={1024}
                  height={760}
                />
              </div>
            </div>
            <div className="relative z-20 mt-6 md:mx-auto md:-mt-12 md:w-[76%]">
              <Shot
                src="/images/logs-filter-pills-zoom.png"
                alt="Filter pills, one equals and one not equals"
                width={1398}
                height={100}
              />
            </div>
          </div>
        </section>

        {/* Live tail */}
        <section className="wrap">
          <FeatureRow
            reverse
            eyebrow="Live tail"
            title={
              <>
                Watch logs arrive <em>in realtime</em>
              </>
            }
            description="Hit Live and the Logs page becomes a live tail. New records stream in at the top every few seconds, with every active filter still applied, so you can follow one service or one severity while you exercise your system."
            bullets={[
              "One click to start tailing, one click to stop",
              "Filters and severity thresholds apply to the tail",
              "Scroll down to read without losing your place",
              "Keeps up under bursts by jumping to the newest records",
            ]}
          >
            <div className="flex items-center justify-center p-10 md:p-16">
              <Image
                src="/images/logs-live-button-zoom.png"
                alt="The Live button, active"
                width={594}
                height={144}
                className="w-full max-w-[420px] h-auto rounded-[10px]"
                style={{
                  border: "1px solid var(--hair)",
                  boxShadow: "0 16px 48px rgba(0, 0, 0, 0.25)",
                }}
              />
            </div>
          </FeatureRow>
        </section>

        {/* Message templates */}
        <section className="wrap py-20">
          <SectionHead
            eyebrow="Message templates"
            title={
              <>
                Structured logs, <em>rendered like structured logs</em>
              </>
            }
            description="Serilog-style message templates are resolved against each record's attributes. Parameter values are highlighted inline, and hovering one shows the parameter it came from. Placeholders without a matching attribute stay literal, so nothing is hidden."
          />
          <div
            className="rounded-[12px] overflow-hidden bg-white"
            style={{ border: "1px solid var(--hair)" }}
          >
            <Image
              src="/images/logs-template-tooltip.png"
              alt="A templated log message with the parameter name shown in a tooltip"
              width={2620}
              height={152}
              className="w-full h-auto block"
            />
          </div>
        </section>

        {/* Expanded row / detail */}
        <section className="wrap">
          <FeatureRow
            eyebrow="Detail"
            title={
              <>
                Expand any line, <em>pivot on anything</em>
              </>
            }
            description="Click a log to see its raw body, the trace and span that produced it, and every attribute across the resource, scope, and log layers. Each attribute doubles as a filter toggle, so one interesting value becomes a search in a single click."
            bullets={[
              "Raw body with linked trace and span IDs",
              "All three attribute layers in one view",
              "One-click filter pivots from any attribute value",
            ]}
            image={{ src: "/images/logs-expanded-row.png", alt: "Expanded log row with attributes", width: 1600, height: 900 }}
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
                JSON, with a 90-day TTL.
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
                  q: "Can I tail logs in real time?",
                  a: "Yes. The Live button on the Logs page polls for new records every few seconds and streams them into the list, with all of your active filters applied. It works with any preset time range.",
                },
                {
                  q: "How long are logs retained?",
                  a: "90 days on Traceway Cloud. ClickHouse-backed storage with daily partitioning and TTL, so retention is enforced automatically without manual cleanup. Self-hosted deployments default to 30 days and are configurable.",
                },
                {
                  q: "Do logs count toward my event limit?",
                  a: "On Traceway Cloud, logs count toward your plan's data allowance, which we size per customer. Reach out and we'll put together a plan for your volume. Self-hosting is unlimited.",
                },
              ]}
            />
          </div>
        </div>
      </section>
    </main>
  );
}
