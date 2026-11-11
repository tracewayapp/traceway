import type { Metadata } from "next";
import { ArrowRight, Check } from "lucide-react";

import { AuroraBackground } from "@/components/aurora-background";
import { FaqList } from "@/components/faq-list";
import { FeatureRow } from "@/components/feature-row";
import { FinalCTA } from "@/components/final-cta";
import { SectionHead } from "@/components/section-head";
import { money, servicesPricing as pricing } from "@/lib/services-pricing";
import { cn } from "@/lib/utils";

const title = "Services · Traceway";
const description =
  "Three ways to get Traceway running: we host it, we roll it out with your team, or we manage it inside your own cloud.";

export const metadata: Metadata = {
  title,
  description,
  alternates: { canonical: "/services" },
  openGraph: { title, description, url: "/services" },
};

const REGISTER_URL = "https://cloud.tracewayapp.com/register";
const DOCS_URL = "https://docs.tracewayapp.com";

type Offer = {
  name: string;
  headline: string;
  who: string;
  items: string[];
  price: string;
  unit?: string;
  priceNote: string;
  cta: { label: string; href: string };
  featured?: boolean;
};

const OFFERS: Offer[] = [
  {
    name: "Traceway Cloud",
    headline: "We host it.",
    who: "For smaller teams that want it running today.",
    items: [
      "Nothing to install or maintain",
      "Set up by your coding agent with our AI skills",
      "Complete with everything you need",
    ],
    price: money(pricing.cloud.premium.monthly),
    unit: "/mo",
    priceNote: "Premium plan. Starter is free.",
    cta: { label: "Start free", href: REGISTER_URL },
  },
  {
    name: "Consulting",
    headline: "We roll it out with you.",
    who: "For teams that want it done right, with experts.",
    items: [
      "Detailed analysis of your architecture and processes",
      "A proposed plan for rolling out Traceway",
      "Help for your team to carry it out",
      "Self-hosted or on our Cloud, under NDA",
    ],
    price: "Scoped to you",
    priceNote: "One project, 30 to 60 days.",
    cta: { label: "Schedule a call", href: pricing.bookingUrl },
    featured: true,
  },
  {
    name: "Dedicated DevOps",
    headline: "We run it in your cloud.",
    who: "For teams that self-host and want it looked after.",
    items: [
      "We monitor and manage your Traceway instance",
      "It stays inside your own cloud, which you pay for",
      "We join your Slack and sign an NDA",
    ],
    price: money(pricing.support.monthly),
    unit: "/mo",
    priceNote: `Optional. ${pricing.support.termMonths}-month contract.`,
    cta: { label: "Schedule a call", href: pricing.bookingUrl },
  },
];

const STEPS = [
  {
    title: "Intro call",
    what: "30 minutes on your team, your stack and what keeps breaking.",
    get: "A clear answer on whether we can help, and a quote.",
  },
  {
    title: "Analysis",
    what: "We go through your architecture and how your team finds and handles incidents.",
    get: "A clear picture of what you’re missing today.",
  },
  {
    title: "Plan",
    what: "We propose the rollout: what to cover first, who gets paged, self-hosted or Cloud.",
    get: "A rollout plan your team can review and own.",
  },
  {
    title: "Rollout",
    what: "We work alongside your team while they carry out the plan.",
    get: "Traceway running, and a team that knows how to use it.",
  },
];

function isExternal(href: string) {
  return href.startsWith("http");
}

function OfferCard({ offer }: { offer: Offer }) {
  return (
    <article
      className={cn(
        "flex flex-col rounded-[14px] border p-7 md:p-8",
        offer.featured
          ? "band-light border-transparent bg-[color:var(--ink-0)]"
          : "border-[color:var(--hair-2)] bg-[color:var(--ink-2)]",
      )}
    >
      <p className="text-[15px] font-semibold text-[color:var(--fg-2)]">{offer.name}</p>
      <h3 className="mt-2 text-[28px] font-semibold leading-tight tracking-tight text-[color:var(--fg-0)]">
        {offer.headline}
      </h3>
      <p className="mt-3 text-[17px] text-[color:var(--fg-1)]">{offer.who}</p>

      <ul className="mt-6 grid gap-3 border-t border-[color:var(--hair-2)] pt-6">
        {offer.items.map((item) => (
          <li key={item} className="grid grid-cols-[20px_1fr] gap-2.5 text-[16px] leading-snug text-[color:var(--fg-0)]">
            <Check className="mt-0.5 h-[18px] w-[18px] text-[color:var(--ok)]" strokeWidth={2.5} aria-hidden />
            <span>{item}</span>
          </li>
        ))}
      </ul>

      <div className="mt-auto pt-8">
        <p className="text-[32px] font-semibold leading-none tracking-tight text-[color:var(--fg-0)]">
          {offer.price}
          {offer.unit ? <span className="text-[16px] font-normal tracking-normal text-[color:var(--fg-2)]">{offer.unit}</span> : null}
        </p>
        <p className="mt-2 text-[15px] text-[color:var(--fg-2)]">{offer.priceNote}</p>
        <a
          href={offer.cta.href}
          className={cn("btn mt-6 w-full justify-center", offer.featured ? "btn-primary" : "btn-ghost")}
          {...(isExternal(offer.cta.href) ? { target: "_blank", rel: "noopener noreferrer" } : {})}
        >
          {offer.cta.label}
          <ArrowRight className="h-4 w-4" />
        </a>
      </div>
    </article>
  );
}

export default function ServicesPage() {
  const support = money(pricing.support.monthly);
  const term = pricing.support.termMonths;

  return (
    <main className="relative">
      <section className="hero hero-product relative">
        <AuroraBackground variant="hero" />
        <div className="wrap relative z-10">
          <h1>Services</h1>
        </div>
      </section>

      <section id="offers" className="wrap pb-16">
        <div className="grid items-stretch gap-5 lg:grid-cols-3">
          {OFFERS.map((offer) => (
            <OfferCard key={offer.name} offer={offer} />
          ))}
        </div>
      </section>

      <section className="wrap pt-4 pb-20 text-center">
        <h2 className="mx-auto max-w-3xl">
          Get Traceway running, <em>with as much help as you want.</em>
        </h2>
        <p className="mx-auto mt-5 max-w-2xl text-[18px] text-[color:var(--fg-1)]">
          Self-hosting is free. If you’d rather have help, we can host it, roll it out with your
          team, or manage it inside your own cloud.
        </p>
        <p className="mt-4 text-[16px] text-[color:var(--fg-2)]">
          Prefer to do it all yourself? Traceway is open source, and your coding agent can set it up.{" "}
          <a href={DOCS_URL} target="_blank" rel="noopener noreferrer" className="underline">
            Read the docs
          </a>
          .
        </p>
      </section>

      <div className="band-light">
        <section className="wrap py-8 md:py-12">
          <FeatureRow
            title={<>Hear about it <em>before your customers do.</em></>}
            description="Most teams find out about an outage from a support email. With checks, alert rules and on-call in place, the right person gets paged first."
            bullets={[
              "Checks on your critical flows, down to every 30 seconds",
              "One alert per issue, not one per error",
              "Pages escalate until someone acknowledges",
            ]}
            image={{ src: "/images/monitors-detail.png", alt: "A Traceway monitor showing the checkout flow as down", width: 3200, height: 2000 }}
          />
          <FeatureRow
            reverse
            title={<>From the alert to <em>the line of code.</em></>}
            description="Errors, traces, logs and replays are linked, so nobody spends the night copying timestamps between tools."
            bullets={[
              "Stack traces that point at the real file and line",
              "A replay of what the customer did before the error",
              "AI agents that investigate through a read-only CLI and MCP",
            ]}
            image={{ src: "/images/exceptions-grouped-ranked.png", alt: "Traceway issues list with errors grouped and ranked", width: 2620, height: 1496 }}
          />
        </section>
      </div>

      <section className="wrap py-16 md:py-24">
        <SectionHead
          title="How consulting works: four steps, 30 to 60 days."
          description="Everything after the intro call is covered by an NDA."
        />
        <div className="hidden grid-cols-[48px_170px_1.25fr_1fr] gap-6 pb-3 text-[14px] font-semibold text-[color:var(--fg-2)] md:grid">
          <span />
          <span>Step</span>
          <span>What happens</span>
          <span>What you get</span>
        </div>
        <ol className="border-t border-[color:var(--hair-2)]">
          {STEPS.map((step, index) => (
            <li
              key={step.title}
              className="grid grid-cols-[40px_1fr] gap-x-3 gap-y-2 border-b border-[color:var(--hair-2)] py-6 md:grid-cols-[48px_170px_1.25fr_1fr] md:gap-6"
            >
              <span className="text-[24px] font-semibold leading-tight text-[color:var(--fg-2)]">{index + 1}</span>
              <h3 className="text-[20px] font-semibold leading-snug text-[color:var(--fg-0)]">{step.title}</h3>
              <p className="col-start-2 text-[17px] text-[color:var(--fg-1)] md:col-start-auto">{step.what}</p>
              <p className="col-start-2 text-[17px] font-medium text-[color:var(--fg-0)] md:col-start-auto">{step.get}</p>
            </li>
          ))}
        </ol>
      </section>

      <section className="wrap pb-8">
        <div className="mx-auto max-w-3xl">
          <SectionHead align="center" title="Quick Q&A" />
          <FaqList
            items={[
              {
                q: "How much does consulting cost?",
                a: "It depends on your team and your architecture. Schedule a call and we’ll scope it with you.",
              },
              {
                q: "Do I need consulting to use Traceway?",
                a: "No. You can set it up with your coding agent and our AI skills. Self-hosting is free and open source, and Cloud has a free Starter plan.",
              },
              {
                q: "Can I get a Dedicated DevOps engineer on Cloud?",
                a: `There’s no need. On Cloud we already run the infrastructure. Dedicated DevOps is an optional ${support} a month on a ${term}-month contract for teams that self-host.`,
              },
              {
                q: "Can I start on Cloud and get consulting later?",
                a: "Yes. Consulting works for teams on Cloud and teams that self-host.",
              },
            ]}
          />
        </div>
      </section>

      <FinalCTA
        title={<>Not sure which one fits? <em>Let’s talk.</em></>}
        description="30 minutes. Tell us about your team and your stack, and we’ll tell you what we’d do."
        primary={{ label: "Schedule a call", href: pricing.bookingUrl, external: true }}
        secondary={{ label: "Download the overview (PDF)", href: "/downloads/traceway-detect-and-fix.pdf" }}
      />
    </main>
  );
}
