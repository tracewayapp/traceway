import Image from "next/image";
import Link from "next/link";
import { ArrowRight, Network, Video, FileCode, TrendingUp } from "lucide-react";

import { Chip } from "@/components/chip";
import { SectionHead } from "@/components/section-head";
import { FeatureRow } from "@/components/feature-row";
import { BentoGrid, BentoCell } from "@/components/bento-grid";
import { FaqList } from "@/components/faq-list";
import { FinalCTA } from "@/components/final-cta";
import { AuroraBackground } from "@/components/aurora-background";

export default function TracesPage() {
  return (
    <main className="relative">
      <section className="hero hero-product relative">
        <AuroraBackground variant="hero" />
        <div className="wrap relative z-10">
          <Chip>
            <Network className="h-3 w-3 inline mr-1" />
            Traces
          </Chip>
          <h1 className="mt-6">
            See the user behind <em>every backend error.</em>
          </h1>
          <p className="hero-sub">
            When a backend service throws an exception, Traceway shows you the
            user&apos;s session replay, the cross-service trace, and the exact
            span that failed. No log-digging. No guessing what happened.
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

      {/* Distributed trace example, absorbed from home */}
      <section className="wrap py-20">
        <SectionHead
          eyebrow="Cross-service"
          title="Trace requests across every service"
          description="Follow a single user action from the browser through your API gateway, backend services, and database calls. See the full picture in one distributed trace."
        />
        <div className="mt-8 max-w-4xl mx-auto">
          <Image
            src="/images/traces-cross-service.png"
            alt="Distributed trace across multiple services"
            width={1600}
            height={500}
            className="w-full h-auto rounded-[12px]"
            style={{ border: "1px solid var(--hair)" }}
          />
        </div>
      </section>

      {/* Full distributed trace screenshot */}
      <section className="wrap">
        <FeatureRow
          eyebrow="Waterfall"
          title={
            <>
              Every span. <em>Every hop.</em>
            </>
          }
          description="Visualize a request as a waterfall: each hop between services shows duration, status, and the exact span where latency or errors appear. Click any span to open its logs, exceptions, and attributes."
          bullets={[
            "Cross-service distributed trace propagation",
            "W3C Trace Context standard headers",
            "Drill-down to span attributes, logs, events",
            "Async workflows (Kafka, RabbitMQ, SQS)",
          ]}
          image={{ src: "/images/traces-spans-waterfall.png", alt: "Span waterfall timing breakdown", width: 1600, height: 500 }}
        />
      </section>

      {/* Session replay + backend errors */}
      <section className="wrap">
        <FeatureRow
          reverse
          eyebrow="Replay"
          title="See what the user did when the backend broke"
          description="Traceway connects frontend session replays to backend exceptions. When your payment service returns a 500, you don't just see the stack trace. You see the user clicking Checkout, filling in their card, and hitting submit."
          bullets={[
            "Frontend replay linked to backend errors",
            "Automatic correlation via trace ID",
            "No manual reproduction needed",
            "Works across browser and server",
          ]}
          image={{ src: "/images/session-replay-viewer.png", alt: "Session replay linked to trace" }}
        />
      </section>

      {/* Two card bento */}
      <section className="wrap py-10">
        <SectionHead
          eyebrow="Deep context"
          title={
            <>
              More than spans, <em>full context</em>
            </>
          }
        />
        <BentoGrid>
          <BentoCell
            size="wide"
            icon={FileCode}
            title="Source map stack trace resolution"
            iconColor="var(--a4)"
          >
            <p>
              Minified JavaScript stack traces are resolved to original source
              files and line numbers automatically. Upload your source maps and
              every frontend error shows readable, actionable traces. Works with
              webpack, esbuild, and Vite.
            </p>
          </BentoCell>
          <BentoCell
            size="tall"
            icon={TrendingUp}
            title="Impact propagation"
            iconColor="var(--a2)"
          >
            <p>
              The Impact Score extends across service boundaries. When an
              upstream service degrades, its impact propagates to every
              downstream consumer, so you fix the root cause, not the symptoms.
            </p>
          </BentoCell>
        </BentoGrid>
      </section>

      <FinalCTA
        title={
          <>
            Trace every hop, <em>not just the first</em>.
          </>
        }
        description="Connect an SDK, propagate one trace ID, and see the whole story."
        primary={{ label: "Get Started", href: "https://docs.tracewayapp.com" }}
      />

      <section className="wrap pt-10 pb-24">
        <div className="max-w-3xl mx-auto">
          <SectionHead align="center" eyebrow="FAQ" title="Questions about traces" />
          <div className="mt-4">
            <FaqList
              items={[
                {
                  q: "How does distributed tracing work?",
                  a: "Traceway propagates a trace ID across every service in a request chain. The frontend SDK generates the ID and passes it to your backend via the traceparent header (W3C Trace Context). Each backend service forwards it to downstream calls. Every span, exception, and session replay recorded with that trace ID is linked together, giving you a complete picture of a single user action across your entire architecture.",
                },
                {
                  q: "Does Traceway connect frontend and backend issues?",
                  a: "Yes. When a backend service returns an error, Traceway links it to the frontend session replay that triggered the request. You see the user's clicks and navigations alongside the server-side stack trace and span waterfall. Both sides are connected automatically via the shared trace ID.",
                },
                {
                  q: "How does distributed tracing connect to session replay?",
                  a: "Traceway's frontend SDK generates a trace ID for each user interaction and passes it to your backend via the traceparent header. When the backend reports a span or exception with that trace ID, Traceway links the frontend session replay to the backend trace automatically.",
                },
                {
                  q: "What protocols does Traceway use for trace propagation?",
                  a: "Traceway supports W3C Trace Context (traceparent/tracestate headers) and is compatible with any OpenTelemetry-instrumented service. If your services already propagate trace context, Traceway picks it up automatically.",
                },
                {
                  q: "Do I need to instrument both frontend and backend?",
                  a: "For the full experience (session replay linked to backend traces), yes. The frontend SDK captures user interactions and the backend middleware captures server-side spans. Both connect via the shared trace ID. However, backend-only distributed tracing works independently.",
                },
                {
                  q: "Does distributed tracing work with message queues and async workflows?",
                  a: "Yes. Traceway uses W3C Trace Context, and OpenTelemetry instrumentation libraries for Kafka, RabbitMQ, SQS, and other message brokers propagate the trace context through message headers automatically. When a consumer processes a message, its spans are linked to the original producer's trace, so an API that publishes to Kafka, which triggers a worker, which calls a downstream service, appears as a single connected trace.",
                },
              ]}
            />
          </div>
        </div>
      </section>
    </main>
  );
}
