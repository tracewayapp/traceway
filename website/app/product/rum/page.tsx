import Link from "next/link";
import type { Metadata } from "next";
import { ArrowRight, Monitor, Gauge, Goal, ScanSearch } from "lucide-react";
import { Chip } from "@/components/chip";
import { SectionHead } from "@/components/section-head";
import { FeatureRow } from "@/components/feature-row";
import { BentoGrid, BentoCell } from "@/components/bento-grid";
import { FaqList } from "@/components/faq-list";
import { FinalCTA } from "@/components/final-cta";
import { AuroraBackground } from "@/components/aurora-background";

export const metadata: Metadata = {
  title: "Real User Monitoring · Traceway",
  description: "Web Vitals, frontend dashboards, release regression alerts, SLOs and session replay. Measure real browser experiences, find who is affected, and investigate the session behind a slow interaction.",
  alternates: { canonical: "https://tracewayapp.com/product/rum" },
};

export default function RumPage() {
  return (
    <main className="relative">
      <section className="hero hero-product relative">
        <AuroraBackground variant="hero" />
        <div className="wrap relative z-10">
          <Chip><Monitor className="h-3 w-3 inline mr-1" />Real User Monitoring</Chip>
          <h1 className="mt-6">Know who is waiting.<br /><em>See what went wrong.</em></h1>
          <p className="hero-sub">A healthy API can still serve a frustrating app. Measure real page loads, interactions and layout shifts. Find the release or device that got slower, give frontend performance an error budget, and follow the evidence into a session replay.</p>
          <div className="hero-cta-row">
            <Link href="https://docs.tracewayapp.com/learn/rum" className="btn btn-accent">Explore RUM <ArrowRight className="h-4 w-4" /></Link>
            <Link href="https://cloud.tracewayapp.com/register" className="btn btn-ghost">Try Traceway Cloud</Link>
          </div>
        </div>
      </section>
      <div className="band-light">
        <section className="wrap">
          <FeatureRow eyebrow="Your first dashboard is already there" title={<>From a new project to <em>real user experience</em></>}
            description="Create a React, Vue, Svelte or jQuery project and Web Performance is ready. As your SDK sends data, see LCP, INP and CLS alongside route timings, blocking time, requests and affected users. Start investigating instead of building the same charts again."
            bullets={["Percentiles show the experience an average can hide", "Rating shares put good and poor experiences side by side", "Break down by release, route, device, OS and country"]}
            image={{ src: "/images/rum-dashboard.png", alt: "Web Performance dashboard populated by real browser visits in a controlled storefront demo" }} />
        </section>
        <section className="wrap">
          <FeatureRow reverse eyebrow="Find the regression, not just the spike" title={<>A release can be fast overall <em>and slow on phones</em></>}
            description="Narrow the dashboard to a release, operating system or country. Compare the same populations before blaming a rollout. Release regression alerts account for sample size and can compare within a segment, so a device-mix change is less likely to masquerade as a code regression."
            bullets={["LCP for loading, INP for interaction, CLS for visual stability", "Minimum effective observations before percentile alerts", "Device-mix adjustment or explicit segment comparisons"]}
            image={{ src: "/images/rum-release.png", alt: "Web Performance filtered to release 0.2.0 and Android in the demo" }} />
        </section>
        <section className="wrap">
          <FeatureRow eyebrow="Make frontend quality a promise" title={<>Give the experience <em>an error budget</em></>}
            description="Define good as an interaction at or below 200 ms, then choose the share your product promises to meet. Every observation counts toward the objective. Split by release to see which rollout is spending the budget, and use burn-rate alerts when the degradation needs attention."
            bullets={["Objectives over observed events, rather than averages of averages", "A separate budget for each release or selected population", "Slow interactions and crash-free sessions answer different questions"]}
            image={{ src: "/images/rum-slos.png", alt: "Frontend interaction SLOs: demo release 0.1.0 meets its target while intentionally slowed release 0.2.0 breaches it" }}
            cta={<Link href="/product/slos" className="btn btn-ghost">Explore frontend SLOs <ArrowRight className="h-4 w-4" /></Link>} />
        </section>
        <section className="wrap">
          <FeatureRow reverse eyebrow="Session replay belongs in the investigation" title={<>From a slow value <em>to the session behind it</em></>}
            description="Open a slow-session exemplar from a chart to find the page and, when available, the element behind a poor observation. Inspect the session timeline, errors and network activity, then watch captured replay to understand what the user experienced. Replay adds context to the population-level signal."
            bullets={["Slow observations link to sampled session exemplars", "Readable errors with source maps", "DOM replay with recording and masking controls"]}
            image={{ src: "/images/rum-slow-session.png", alt: "Session investigation opened from a poor INP observation in the frontend dashboard" }}
            cta={<Link href="https://docs.tracewayapp.com/learn/sessions" className="btn btn-ghost">Sessions and replay <ArrowRight className="h-4 w-4" /></Link>} />
        </section>
        <section className="wrap">
          <FeatureRow eyebrow="Capture the journey you need to understand" title={<>Choose the actions. <em>Keep the explanation.</em></>}
            description="Build a sequence of clicks, hovers, navigations and log messages in the visual editor. When the journey completes, the browser requests a replay capture. Find those visits through readable trigger tags, including multiple reasons on the same session."
            bullets={["Configure ordered steps without writing an expression", "Capture bounded history with cooldowns and per-session limits", "Filter by trigger and preserve its historical name and sequence"]}
            image={{ src: "/images/rum-sequence-builder.png", alt: "Traceway visual editor configuring click, navigation and log steps for a replay capture" }}
            cta={<Link href="https://docs.tracewayapp.com/learn/remote-config/trigger-sequences" className="btn btn-ghost">Build a recording trigger <ArrowRight className="h-4 w-4" /></Link>} />
        </section>
        <section className="wrap">
          <FeatureRow reverse eyebrow="Every recording has context" title={<>Find the sessions <em>that followed your rule</em></>}
            description="A session can match more than one journey. Its trigger tags explain which sequences requested capture, and each tag opens a shareable filter. Historical reasons survive rule edits and deletion; replay availability is shown separately."
            image={{ src: "/images/rum-sequence-sessions.png", alt: "Session list showing multiple recording trigger tags with readable action sequences" }} />
        </section>
        <section className="wrap py-10">
          <SectionHead eyebrow="Why it helps" title={<>Measure, prioritize, <em>then explain</em></>} />
          <BentoGrid>
            <BentoCell size="wide" icon={Gauge} title="Prioritize by observed impact" iconColor="var(--a1)"><p>Pair the severity of a slowdown with observation volume and affected browser profiles. A bad percentile on one rarely visited page and a widespread checkout delay call for different responses.</p></BentoCell>
            <BentoCell size="tall" icon={Goal} title="Share a concrete quality target" iconColor="var(--ok)"><p>An objective turns “the app feels slow” into a measurable population, threshold and budget. Product and engineering can discuss the same evidence.</p></BentoCell>
            <BentoCell size="wide" icon={ScanSearch} title="Keep the evidence connected" iconColor="var(--a2)"><p>Metrics tell you where to look. Sessions, replay and errors help explain why. You can move from a release comparison to an individual experience without starting a separate investigation.</p></BentoCell>
          </BentoGrid>
          <p className="mt-8 text-sm text-muted-foreground">Screenshots show controlled storefront and user-directory demos with real browser telemetry. Release 0.2.0 is intentionally slowed to demonstrate regressions; these are not customer benchmarks.</p>
        </section>
      </div>
      <FinalCTA title={<>Ship a frontend <em>you can measure.</em></>} description="Frontend metrics, objectives, alerts and replay in the same investigation workflow. Available with Cloud or a self-hosted Traceway server." primary={{ label: "Read the RUM guide", href: "https://docs.tracewayapp.com/learn/rum" }} />
      <section className="wrap pt-10 pb-24"><div className="max-w-3xl mx-auto">
        <SectionHead align="center" eyebrow="FAQ" title="RUM, metrics and replay" />
        <FaqList items={[
          { q: "Is RUM the same as session replay?", a: "Replay shows a recorded experience. RUM also measures performance across the observed population, compares releases and devices, and supports objectives and alerts. Replay is one investigation tool within RUM." },
          { q: "Do metrics require recording every session?", a: "No. Browser performance metrics are aggregated on the device and uploaded as distributions or counters. Replay collection has separate controls. A metric can exist without a replay, and a slow-session exemplar does not guarantee a recording." },
          { q: "Which SDK version do I need?", a: "The aggregated browser-metrics path requires SDK 2.1 or later and a compatible server. Use the installation guide for the current release channel; the 2.x browser documentation currently uses the beta tag. Existing 2.0 performance events can also populate metrics on the server." },
          { q: "What about privacy and sampling?", a: "Use normalized routes and low-cardinality dimensions, never user IDs or emails as metric labels. Configure replay masking and collection for your application. Sampled totals and histogram percentiles are estimates; blocked telemetry and unsupported browser APIs affect coverage." },
        ]} />
      </div></section>
    </main>
  );
}
