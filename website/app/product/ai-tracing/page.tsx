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
            Monitor LLM costs, token usage, latency, conversations, and tool
            calls across every provider. Group calls into conversations, break
            them down per user, and flag the ones that need attention. Works
            with OpenRouter, OpenAI, Anthropic, and any OpenTelemetry-compatible
            provider.
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
            title="Replay every conversation, tool calls included"
            description="See the exact prompt sent to the model and the full response it generated. Multi-turn conversations render as one chat timeline, with tool calls shown inline: function name, arguments, and results. Debug unexpected model behavior, catch hallucinations, and understand what your AI agents are actually doing."
            bullets={[
              "Full prompt + completion stored and rendered as chat",
              "Tool calls rendered with arguments and paired results",
              "Sub-agents traced under their own name in the same conversation",
              "Raw JSON view for debugging edge cases",
              "Privacy mode available to exclude conversation content",
            ]}
            image={{ src: "/images/ai-conversation-detail.png", alt: "Conversation replay with tool calls" }}
          />
        </section>

        {/* Conversations & users */}
        <section className="wrap">
          <FeatureRow
            eyebrow="Conversations"
            title={
              <>
                Find the outliers, <em>per conversation and per user</em>
              </>
            }
            description="Calls group into conversations automatically. See turns, cost, tokens, and tool usage per conversation, and roll it all up per user: how long conversations run, what they cost, and who your heaviest users are. Anything above the 95th percentile is highlighted."
            bullets={[
              "Turns, cost, tokens, and tool calls per conversation",
              "Per-user analytics: median conversation length, cost per conversation",
              "Filter by user, model, or specific tool with removable filter pills",
              "P95 outlier highlighting on cost and conversation length",
            ]}
            image={{ src: "/images/ai-conversations.png", alt: "AI conversations analytics" }}
          />
        </section>

        {/* Content flags */}
        <section className="wrap">
          <FeatureRow
            reverse
            eyebrow="Guardrails"
            title="Flag conversations that need attention"
            description="Every prompt and completion is scanned at ingest against built-in profanity lists in seven languages plus your own custom terms: competitor names, refund phrases, compliance keywords. Flagged conversations get a badge, a filter, and can trigger alerts."
            bullets={[
              "Built-in profanity packs: English, German, Spanish, French, Italian, Portuguese, Serbian",
              "Custom per-project terms for anything beyond profanity",
              "Indexed once at ingestion, searchable and filterable instantly",
              "Alert rules for flagged content and runaway conversation cost",
            ]}
            image={{ src: "/images/ai-flagged-terms.png", alt: "Flagged term configuration" }}
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
                  q: "How does per-user cost and conversation tracking work?",
                  a: (
                    <>
                      <p>
                        Traceway keys per-customer analytics on the{" "}
                        <code>user.id</code> span attribute (with OpenRouter,
                        just pass the <code>user</code> field in your requests).
                        Set it to a stable identifier for the end user of your
                        product: your internal account id, a tenant id, or an
                        email. The same user must carry the same value across
                        all their conversations.
                      </p>
                      <p>
                        Never put session ids or random values there — that is
                        what the conversation id is for. With a stable id, the
                        Users view shows conversation count, median conversation
                        length, and cost per conversation for every customer.
                        If PII must stay out of telemetry, use an internal id or
                        a hash; the analytics only need stability.
                      </p>
                    </>
                  ),
                },
                {
                  q: "Can I find conversations containing specific words, like profanity?",
                  a: "Yes. Traceway scans every prompt and completion at ingestion against built-in profanity lists (seven languages) plus custom terms you configure per project. Matches are stored on the call, so flagged conversations are instantly filterable and searchable, and an alert rule can notify you the moment flagged content appears. Because scanning happens at ingest, there is no expensive query-time content search.",
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
