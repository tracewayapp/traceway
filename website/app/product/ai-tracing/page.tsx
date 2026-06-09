import Image from "next/image";
import Link from "next/link";
import { ArrowRight, DollarSign, MessageSquareText, BarChart3, Workflow } from "lucide-react";

import { Chip } from "@/components/chip";
import { SectionHead } from "@/components/section-head";
import { FeatureRow } from "@/components/feature-row";
import { FaqList } from "@/components/faq-list";
import { FinalCTA } from "@/components/final-cta";
import { AuroraBackground } from "@/components/aurora-background";
import { Eyebrow } from "@/components/eyebrow";

export default function AiTracingPage() {
  return (
    <main className="relative">
      <section className="hero hero-product relative">
        <AuroraBackground variant="hero" />
        <div className="wrap relative z-10">
          <Chip>
            <Workflow className="h-3 w-3 inline mr-1" />
            AI Tracing
          </Chip>
          <h1 className="mt-6">
            See every AI call, <em>its cost, and its conversation.</em>
          </h1>
          <p className="hero-sub">
            Monitor LLM costs, token usage, latency, and conversations across
            every provider. Works with OpenRouter, OpenAI, Anthropic, and any
            OpenTelemetry-compatible provider.
          </p>
          <div className="hero-cta-row">
            <Link href="https://docs.tracewayapp.com/client/openrouter" className="btn btn-accent">
              Get Started <ArrowRight className="h-4 w-4" />
            </Link>
            <Link href="https://cloud.tracewayapp.com/register" className="btn btn-ghost">
              Try Traceway Cloud
            </Link>
          </div>
        </div>
      </section>

      {/* Cost */}
      {/* WHITE BAND: feature sections render on white */}
      <div className="band-light">
        <section className="wrap">
          <FeatureRow
            eyebrow="Cost"
            title={
              <>
                Know exactly <em>what AI costs you</em>
              </>
            }
            description="Every AI call tracked with input cost, output cost, and total cost. See breakdowns per agent, per model, and per time period. Spot cost spikes before they hit your invoice."
            bullets={[
              "Per-call cost tracking with input / output breakdown",
              "Aggregated cost per agent and model",
              "Token usage with cached and reasoning token tracking",
              "Anomaly detection on cost per call",
            ]}
            image={{ src: "/images/ai-traces-cost.png", alt: "AI cost dashboard" }}
          />
        </section>

        {/* Conversation */}
        <section className="wrap">
          <FeatureRow
            reverse
            eyebrow="Conversation"
            title="Replay every conversation"
            description="See the exact prompt sent to the model and the full response it generated. Debug unexpected model behavior, catch hallucinations, and understand what your AI agents are actually doing."
            bullets={[
              "Full prompt + completion stored and rendered as chat",
              "Raw JSON view for debugging edge cases",
              "Privacy mode available to exclude conversation content",
              "Search and filter by prompt content",
            ]}
            image={{ src: "/images/ai-traces-conversation.png", alt: "Conversation replay" }}
          />
        </section>

        {/* Performance */}
        <section className="wrap">
          <FeatureRow
            eyebrow="Latency"
            title="Understand AI latency at every percentile"
            description="AI calls have wildly variable latency. See P50 and P95 duration breakdowns per agent, identify which models or providers are causing slowdowns, and track performance over time."
            bullets={[
              "P50 / P95 latency per agent and model",
              "Drill down from agent to individual calls",
              "Throughput and token usage trends",
              "Compare providers side-by-side",
            ]}
            image={{ src: "/images/ai-traces-latency.png", alt: "AI latency dashboard" }}
          />
        </section>

        {/* Zero code */}
        <section className="wrap py-16 text-center">
          <div className="max-w-3xl mx-auto flex flex-col items-center gap-5">
            <Eyebrow>Instrument-free</Eyebrow>
            <h2>Zero code changes required</h2>
            <p style={{ color: "var(--fg-1)", fontSize: 17 }}>
              If you&apos;re using OpenRouter, enable Observability in your
              settings and point it at Traceway. That&apos;s it. For other
              providers, any OpenTelemetry-instrumented AI call with{" "}
              <code>gen_ai.*</code> attributes is automatically captured.
            </p>
            <div className="flex flex-wrap items-center justify-center gap-10 pt-5 opacity-80">
              <Image
                src="/images/frameworks/openrouter.png"
                alt="OpenRouter"
                width={40}
                height={40}
                className="h-8 w-auto opacity-80 hover:opacity-100 transition-all"
              />
              <Image
                src="/images/frameworks/otel.png"
                alt="OpenTelemetry"
                width={40}
                height={40}
                className="h-8 w-auto opacity-80 hover:opacity-100 transition-all"
              />
            </div>
          </div>
        </section>
      </div>

      <FinalCTA
        title={
          <>
            Trace AI <em>like any other service</em>
          </>
        }
        description="Costs, tokens, latency, and conversations, all in one place."
        primary={{
          label: "Read the AI Tracing docs",
          href: "https://docs.tracewayapp.com/client/openrouter",
        }}
      />

      <section className="wrap pt-10 pb-24">
        <div className="max-w-3xl mx-auto">
          <SectionHead align="center" eyebrow="FAQ" title="Questions about AI tracing" />
          <div className="mt-4">
            <FaqList
              items={[
                {
                  q: "Why do I need AI observability?",
                  a: "AI API calls are expensive and unpredictable. A single prompt with a large context window can cost 100x more than average. Without observability, cost spikes go unnoticed until the invoice arrives. AI tracing gives you per-call visibility into costs, token usage, latency, and the actual conversations happening between your app and AI models.",
                },
                {
                  q: "Which AI providers are supported?",
                  a: (
                    <>
                      <p>
                        Any provider or gateway that exports OTLP traces with{" "}
                        <code>gen_ai.*</code> semantic convention attributes.
                      </p>
                      <p>
                        OpenRouter has built-in support. Just enable
                        Observability in your settings. For other providers, use
                        any OpenTelemetry SDK to instrument your AI calls and
                        send them to Traceway.
                      </p>
                    </>
                  ),
                },
                {
                  q: "Is the conversation content stored securely?",
                  a: "Conversation content (prompts and completions) is stored separately from trace metadata in object storage (S3 or local filesystem). If you're self-hosting, the data never leaves your infrastructure. OpenRouter also offers a Privacy Mode that excludes conversation content entirely, sending only metadata like model, tokens, and costs.",
                },
                {
                  q: "How does this work with OpenRouter?",
                  a: "OpenRouter has a built-in Observability feature that broadcasts OTLP traces for every LLM call. You add Traceway as an 'OpenTelemetry Collector' destination in your OpenRouter settings with your Traceway endpoint and project token. No code changes needed.",
                },
                {
                  q: "Can I track costs across multiple models?",
                  a: (
                    <>
                      <p>
                        Yes. AI traces capture the specific model used for each
                        call (e.g., <code>openai/gpt-4-turbo</code>,{" "}
                        <code>anthropic/claude-3-opus</code>).
                      </p>
                      <p>
                        You can see cost breakdowns per model, compare token
                        efficiency across providers, and identify which models
                        give you the best value.
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
