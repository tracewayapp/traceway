import type { Metadata } from "next";
import Link from "next/link";
import { ArrowRight, Braces, GitBranch, Network, Radio } from "lucide-react";

import { Chip } from "@/components/chip";
import { SectionHead } from "@/components/section-head";
import { FaqList } from "@/components/faq-list";
import { FinalCTA } from "@/components/final-cta";
import { AuroraBackground } from "@/components/aurora-background";
import { TracingShowcase } from "@/components/tracing-showcase";
import { ScreenshotZoom } from "@/components/screenshot-zoom";
import { tracingScreenshots } from "@/lib/tracing-screenshots";

export const metadata: Metadata = {
  title: "OpenTelemetry Tracing & Span Explorer | Traceway",
  description: "Search OpenTelemetry spans by service, duration, status and attributes. Follow a trace across services, inspect its waterfall and read correlated logs in Traceway.",
};

const tracingDocs = "https://docs.tracewayapp.com/client/otel/traces";

export default function TracesPage() {
  return (
    <main className="relative">
      <section className="hero hero-product relative">
        <AuroraBackground variant="hero" />
        <div className="wrap relative z-10">
          <Chip><Network className="mr-1 inline h-3 w-3" />OpenTelemetry tracing</Chip>
          <h1 className="mt-6 max-w-5xl">
            Search any span.<br /><em>Follow the whole trace.</em>
          </h1>
          <p className="hero-sub">
            Find the slow query, failed request or background job. Open its trace
            to see the services involved, where the time went, and what the logs
            say—all connected by the original OpenTelemetry trace ID.
          </p>
          <div className="hero-cta-row">
            <Link href={tracingDocs} className="btn btn-accent">
              Connect OpenTelemetry <ArrowRight className="h-4 w-4" />
            </Link>
            <Link href="https://cloud.tracewayapp.com/register" className="btn btn-ghost">
              Try Traceway Cloud
            </Link>
          </div>
        </div>
      </section>

      <section className="wrap pb-16 sm:pb-20" aria-label="Explore tracing in Traceway">
        <TracingShowcase />
      </section>

      <div className="band-light">
        <section className="wrap py-16 sm:py-20" aria-label="OpenTelemetry features">
          <SectionHead
            eyebrow="OpenTelemetry throughout"
            title={<>Keep the context <em>your tools already send.</em></>}
            description="Send traces from your existing OpenTelemetry SDK or Collector over OTLP/HTTP. Your services keep their trace IDs, span IDs and parent relationships."
          />
          <div className="grid gap-10 md:grid-cols-3">
            {[
              {
                icon: Radio,
                title: "Search beyond HTTP",
                text: "Database calls, internal operations, producers and consumers are all searchable spans. Filter the work itself, even when it has no endpoint or task page.",
              },
              {
                icon: GitBranch,
                title: "Follow service boundaries",
                text: "See one trace across services and the projects you can access in your organization. Expand branches, spot errors and jump straight to a selected span.",
              },
              {
                icon: Braces,
                title: "Preserve the OTel detail",
                text: "Original OTLP payloads retain typed attributes, resource and scope metadata, events and span links. Retrieve a stored span through the OTLP export API when you need its full payload.",
              },
            ].map(({ icon: Icon, title, text }) => (
              <div key={title} className="border-t border-hair-2 pt-6">
                <Icon className="mb-5 h-5 w-5 text-a2" aria-hidden="true" />
                <h3 className="text-xl">{title}</h3>
                <p className="mt-3 text-sm leading-relaxed text-fg-2">{text}</p>
              </div>
            ))}
          </div>
        </section>

        <section className="wrap pb-16 sm:pb-20" aria-label="Correlated logs">
          <SectionHead
            eyebrow="Correlated logs"
            title={<>The request failed. <em>Here is what it logged.</em></>}
            description="Read logs carrying the same trace ID alongside the waterfall. See the emitting service and span, then expand a record for its attributes. Logs stay scoped to the selected project."
          />
          <figure className="overflow-hidden rounded-xl border border-hair-2 bg-ink-1">
            <ScreenshotZoom images={tracingScreenshots} index={3} />
            <figcaption className="px-5 py-4 text-sm leading-relaxed text-fg-2">
              The same checkout shown above: a 950 ms payment timeout, followed by
              a failed-order event and the gateway&apos;s 502 response. Demo data.
            </figcaption>
          </figure>
        </section>

        <section className="wrap border-t border-hair py-16" aria-label="Browser tracing">
          <div className="grid items-start gap-8 md:grid-cols-2 md:gap-16">
            <h2>From the browser <em>to the backend.</em></h2>
            <div>
              <p className="leading-relaxed text-fg-2">
                The Traceway browser SDK propagates W3C trace context on configured
                requests. When your backend instrumentation continues that context,
                captured errors and sessions can carry the same trace ID. Add session
                replay to see what the user was doing alongside the technical context.
              </p>
              <Link href="https://docs.tracewayapp.com/client/js-sdk/distributed-tracing" className="mt-6 inline-flex items-center gap-2 text-sm font-medium text-a2 underline-offset-4 hover:underline">
                Set up browser-to-backend tracing <ArrowRight className="h-4 w-4" />
              </Link>
            </div>
          </div>
        </section>
      </div>

      <section className="wrap py-16 sm:py-20" aria-label="Tracing questions">
        <div className="mx-auto max-w-3xl">
          <SectionHead align="center" eyebrow="FAQ" title="Questions about tracing" />
          <FaqList items={[
            {
              q: "Can I use my existing OpenTelemetry instrumentation?",
              a: "Yes. Send traces with an OpenTelemetry SDK or Collector using OTLP over HTTP, in protobuf or JSON format. Traceway keeps the original trace and span IDs. Backend tracing works independently of the Traceway browser SDK.",
            },
            {
              q: "What can I search in the span explorer?",
              a: "Search received spans by name, service, span kind, status, minimum or maximum duration, trace ID and exact attribute values within a time range. HTTP calls, database queries, internal spans and messaging operations are all available. Open a result to view its trace with that span selected.",
            },
            {
              q: "How are spans connected across services?",
              a: "Your instrumentation propagates W3C trace context between services. Spans in the same trace share a trace ID, and parent span IDs describe their relationships. Traceway assembles that trace across the projects you can access in the selected organization.",
            },
            {
              q: "Are span links the same as parent-child relationships?",
              a: "No. OpenTelemetry span links can reference spans in the same trace or a different trace, for example in a batch or asynchronous workflow. Traceway preserves those links in the original OTLP payload and export. They do not merge separate traces into one waterfall. Whether a queue consumer continues a trace or links to it depends on your instrumentation.",
            },
            {
              q: "How do logs connect to a trace?",
              a: "Export logs with a trace ID and, when available, a span ID. The trace page shows matching logs in the selected project and surrounding time window, with their service and span names. A span ID lets you narrow a log search to a specific operation.",
            },
            {
              q: "Can I connect browser errors and session replay?",
              a: "Yes. Configure W3C propagation in the Traceway browser SDK and context extraction in your backend instrumentation. Errors and sessions carrying that trace ID can be correlated with the backend trace. For cross-origin requests, allow the destination in the SDK and permit traceparent and tracestate in your server's CORS policy.",
            },
          ]} />
        </div>
      </section>

      <FinalCTA
        title={<>Bring your spans. <em>See the request.</em></>}
        description="Connect your OpenTelemetry exporter and start with the span explorer."
        primary={{ label: "Connect OpenTelemetry", href: tracingDocs }}
        secondary={{ label: "Try Traceway Cloud", href: "https://cloud.tracewayapp.com/register" }}
      />
    </main>
  );
}
