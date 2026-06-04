import Link from "next/link";
import { ArrowRight, Cloud as CloudIcon } from "lucide-react";

import { Chip } from "@/components/chip";
import { SectionHead } from "@/components/section-head";
import { FaqList } from "@/components/faq-list";
import { FinalCTA } from "@/components/final-cta";
import { AuroraBackground } from "@/components/aurora-background";
import { PricingCalculator } from "@/components/pricing-calculator";
import { CostComparison } from "@/components/cost-comparison";
import { getCalendlyUrl } from "@/lib/calendly";

export default function CloudPage() {
  return (
    <main className="relative">
      <section className="hero hero-product relative">
        <AuroraBackground variant="hero" />
        <div className="wrap relative z-10">
          <Chip>
            <CloudIcon className="h-3 w-3 inline mr-1" />
            Traceway Cloud
          </Chip>
          <h1 className="mt-6">
            Managed Traceway <em>for teams.</em>
          </h1>
          <p className="hero-sub">
            Focus on shipping features, not managing infrastructure. All the
            power of Traceway with zero maintenance. Same open-source code,
            managed by us.
          </p>
          <div className="hero-cta-row">
            <Link href="https://cloud.tracewayapp.com/register" className="btn btn-accent">
              Start Free <ArrowRight className="h-4 w-4" />
            </Link>
            <Link href="https://docs.tracewayapp.com/cloud" className="btn btn-ghost">
              How it works
            </Link>
          </div>
        </div>
      </section>

      {/* Pricing */}
      <section className="wrap py-20">
        <SectionHead
          eyebrow="Pricing"
          title="Simple, predictable pricing"
          description="Start free and scale as you grow. No credit card required for the Starter plan."
        />
        <div className="mt-8">
          <PricingCalculator />
        </div>
      </section>

      {/* Cost comparison / cost advantage, absorbed from home */}
      <section className="wrap py-10" id="cost-mount" data-cost-mount>
        <SectionHead
          eyebrow="Cost"
          title={
            <>
              Designed for efficiency. <em>Built to lower your cloud bill.</em>
            </>
          }
          description="Traceway runs lean. ClickHouse columnar storage compresses 1 million daily events into ~2-3 GB per month. Postgres is used for efficient user and organization storage."
        />
        <div className="mt-8">
          <CostComparison />
        </div>
        <div className="mt-10 flex flex-col sm:flex-row items-center justify-center gap-3">
          <Link href="https://cloud.tracewayapp.com/register" className="btn btn-accent">
            Start on Cloud <ArrowRight className="h-4 w-4" />
          </Link>
          <Link href="https://docs.tracewayapp.com" className="btn btn-ghost">
            Self-host for free
          </Link>
        </div>
      </section>

      <FinalCTA
        title={
          <>
            Observability <em>without the infra tax</em>
          </>
        }
        description="Run on us, or run it yourself. Same features, no asterisks."
        primary={{
          label: "Start Free",
          href: "https://cloud.tracewayapp.com/register",
        }}
        secondary={{
          label: "Contact Sales",
          href: getCalendlyUrl(),
        }}
      />

      {/* FAQ, includes absorbed from home */}
      <section className="wrap pt-10 pb-24">
        <div className="max-w-3xl mx-auto">
          <SectionHead align="center" eyebrow="FAQ" title="Cloud & pricing FAQ" />
          <div className="mt-4">
            <FaqList
              items={[
                {
                  q: "How does Traceway compare to Datadog / New Relic?",
                  a: (
                    <>
                      <p>
                        Datadog and New Relic charge per host, per event, or per
                        GB ingested, and bills can spike unpredictably as
                        traffic grows. Traceway Cloud has fixed-price tiers.
                        At the Enterprise level, 200 million monthly events
                        cost $499.99 ($0.0000025 per event) with no overage
                        charges. Self-hosted Traceway has zero licensing cost.
                      </p>
                      <p>
                        Architecturally, Traceway uses ClickHouse columnar
                        storage that compresses 1 million daily events into
                        ~2-3 GB per month, keeping infrastructure costs low even
                        at high volume. Feature-wise, Traceway includes
                        endpoint performance analytics, exception tracking with
                        automatic grouping and ranking, session replay,
                        distributed tracing, metrics, logs, and AI
                        observability, all in one tool. Datadog and New Relic
                        split these across separate products, each with its
                        own billing meter.
                      </p>
                    </>
                  ),
                },
                {
                  q: "Is Traceway really free to self-host?",
                  a: (
                    <>
                      <p>
                        Yes. Traceway is 100% open source with no feature
                        gating. Every feature available on Traceway Cloud works
                        identically when self-hosted. Deploy with{" "}
                        <code>docker compose up -d</code> and you&apos;re
                        running.
                      </p>
                    </>
                  ),
                },
                {
                  q: "Will my bill ever increase unexpectedly?",
                  a: "No. Every Traceway Cloud plan has a fixed monthly price with no overage charges. If you approach your included volume, we notify you in advance so you can decide whether to upgrade. Your bill will never increase without your explicit approval. This applies to every tier, from Starter to Enterprise.",
                },
                {
                  q: "What support do Cloud customers get?",
                  a: "All Cloud customers on a paid plan can open GitHub issues that are triaged with highest priority by our engineering team. You talk directly to the people who build Traceway, with no help desk routing. Enterprise+ customers also receive a shared Slack channel with direct access to the team. Self-hosted users are welcome to open GitHub issues and participate in community discussions. We actively monitor and respond.",
                },
                {
                  q: "Are there overage charges?",
                  a: "No. Every plan has a fixed monthly price. If you approach your included volume, we notify you in advance so you can decide whether to upgrade. What you see on the pricing table is what you pay. No metered billing, no surprise line items, no usage-based surcharges.",
                },
                {
                  q: "How does Traceway Cloud compare on cost at scale?",
                  a: "At the Enterprise tier, 200 million monthly events cost $499.99, which is $0.0000025 per event. Competitors like Datadog and Sentry charge orders of magnitude more at the same volume, often with additional per-host, per-seat, or overage fees on top. Enterprise+ offers even cheaper per-event pricing with a dedicated SRE and shared Slack channel.",
                },
                {
                  q: "What counts as an event?",
                  a: "An event is any single issue (exception), HTTP request, or background task run that Traceway ingests. Session replays, distributed trace spans, custom metrics, and logs are included at no additional cost and do not count toward your event volume. For example, if your application handles 50,000 HTTP requests and encounters 200 exceptions in a month, that is 50,200 events.",
                },
                {
                  q: "Can I migrate from Cloud to Self-Hosted later?",
                  a: "Yes. Since the underlying software is the same, we can work with you to export your data and migrate to a self-hosted instance at any time. You are never locked into our cloud platform.",
                },
                {
                  q: "Is the open-source version limited?",
                  a: "No. The code is 100% open source and fully featured. We do not gate features behind the Cloud version. Cloud exists solely for convenience and for users who prefer a managed service over self-hosting.",
                },
                {
                  q: "Why use Traceway Cloud?",
                  a: "Cloud is for teams that don't want to self-host. We run the exact same open-source code but manage the infrastructure, updates, and backups for you. Focus on shipping features without maintaining an observability stack.",
                },
              ]}
            />
          </div>
        </div>
      </section>
    </main>
  );
}
