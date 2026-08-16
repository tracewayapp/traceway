import Link from "next/link";
import { ArrowRight, Cloud as CloudIcon } from "lucide-react";

import { Chip } from "@/components/chip";
import { SectionHead } from "@/components/section-head";
import { FaqList } from "@/components/faq-list";
import { FinalCTA } from "@/components/final-cta";
import { AuroraBackground } from "@/components/aurora-background";
import { ComplianceStrip } from "@/components/compliance-strip";
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
            <Link
              href="https://cloud.tracewayapp.com/register"
              className="btn btn-accent"
            >
              Start Free <ArrowRight className="h-4 w-4" />
            </Link>
          </div>
        </div>
      </section>

      {/* Pricing */}
      {/* WHITE BAND: pricing + compliance render on white */}
      <div className="band-light">
        <section className="wrap md:py-20! py-10!">
          <SectionHead
            align="center"
            eyebrow="Pricing"
            title={
              <>
                Pricing that fits <em>your team.</em>
              </>
            }
            description="We price Traceway Cloud per customer, sized to your actual volume and needs. Start free, and when you're ready, we'll put together a plan with no per-host or per-seat fees."
          />
          <div className="mt-2 flex flex-wrap justify-center gap-3">
            <Link href={getCalendlyUrl()} className="btn btn-accent">
              Talk to us about pricing <ArrowRight className="h-4 w-4" />
            </Link>
            <Link
              href="https://cloud.tracewayapp.com/register"
              className="btn btn-ghost"
            >
              Start Free
            </Link>
          </div>
        </section>

        {/* Security & compliance */}
        <section className="wrap pb-20">
          <SectionHead
            align="center"
            eyebrow="Security & compliance"
            title={
              <>
                Built to <em>enterprise standards.</em>
              </>
            }
            description="Independent third-party audits are underway. SOC 2 Type II and ISO 27001 reports will be available on completion; reach out for current status under NDA."
          />
          <div className="mx-auto mt-2 max-w-2xl">
            <ComplianceStrip />
          </div>
        </section>
      </div>

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
          <SectionHead align="center" eyebrow="FAQ" title="Cloud FAQ" />
          <div className="mt-4">
            <FaqList
              items={[
                {
                  q: "How does pricing work?",
                  a: "We price per customer. Every plan starts from two simple meters, a monthly allowance for exceptions and one for ingested data (logs, traces, spans, metrics, session replay), and we size both to your workload. Book a call, tell us your volume, and we'll put together a plan. HTTP requests and background task runs are never metered, and there are no per-host or per-seat fees.",
                },
                {
                  q: "Why is Traceway so cost-effective?",
                  a: (
                    <>
                      <p>
                        There are no per-host or per-seat fees layered on top of
                        your plan, and self-hosted Traceway has zero licensing
                        cost.
                      </p>
                      <p>
                        Architecturally, Traceway uses ClickHouse columnar
                        storage that compresses 1 million daily events into ~2-3
                        GB per month, keeping costs low even at high volume.
                        Everything is in one tool: endpoint performance
                        analytics, exception tracking with automatic grouping
                        and ranking, session replay, distributed tracing,
                        metrics, logs, and AI observability, with a single bill
                        instead of a separate meter per product.
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
                  a: "No. Your pricing is agreed with you up front. If your usage approaches what we agreed, we reach out in advance so you can decide how to proceed. There are no per-host or per-seat surprises, and we never apply a new rate without telling you first.",
                },
                {
                  q: "What support do Cloud customers get?",
                  a: "All Cloud customers on a paid plan can open GitHub issues that are triaged with highest priority by our engineering team. You talk directly to the people who build Traceway, with no help desk routing. Larger plans also include a shared Slack channel with direct access to the team. Self-hosted users are welcome to open GitHub issues and participate in community discussions. We actively monitor and respond.",
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
