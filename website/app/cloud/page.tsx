import { Cloud as CloudIcon } from "lucide-react";

import { Chip } from "@/components/chip";
import { SectionHead } from "@/components/section-head";
import { FaqList } from "@/components/faq-list";
import { AuroraBackground } from "@/components/aurora-background";
import { PricingPlans } from "@/components/pricing-plans";
import { ComplianceStrip } from "@/components/compliance-strip";

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
            Simple pricing <em>for teams.</em>
          </h1>
          <p className="hero-sub">
            The same open-source Traceway, managed by us. Start free and
            upgrade when you need more.
          </p>
        </div>
      </section>

      <div className="band-light">
        <section className="wrap md:py-20! py-10!">
          <PricingPlans />
        </section>

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

      <section className="wrap pt-16 pb-24">
        <div className="max-w-3xl mx-auto">
          <SectionHead align="center" eyebrow="FAQ" title="Quick Q&A" />
          <div className="mt-4">
            <FaqList
              items={[
                {
                  q: "What counts toward my limits?",
                  a: "Two things: exceptions, and the data you ingest (logs, traces and metrics). There are no per-host or per-seat fees.",
                },
                {
                  q: "What happens when I reach a limit?",
                  a: "The dashboard warns you at 80%. At the limit, new data for that meter is paused until your next billing period or until you upgrade. On Premium you can turn on overages and keep ingesting data at $0.25 per GB.",
                },
                {
                  q: "How does Enterprise pricing work?",
                  a: "It is a custom plan. Tell us your volume and we size the limits and the price to it, billed monthly or annually.",
                },
                {
                  q: "Can I cancel anytime?",
                  a: "Yes. Cancel from your billing settings and the plan stays active until the end of the period you paid for.",
                },
                {
                  q: "Can I self-host instead?",
                  a: "Yes. Traceway is open source and every feature works the same when you run it yourself, with no license cost.",
                },
              ]}
            />
          </div>
        </div>
      </section>
    </main>
  );
}
