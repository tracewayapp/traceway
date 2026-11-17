import Link from "next/link";
import type { Metadata } from "next";
import {
  ArrowRight, Monitor, ScanSearch, Activity, Route, Users, Bug, Ruler, EyeOff, Server, BellRing,
} from "lucide-react";
import { Chip } from "@/components/chip";
import { SectionHead } from "@/components/section-head";
import { FeatureRow } from "@/components/feature-row";
import { BentoGrid, BentoCell } from "@/components/bento-grid";
import { FaqList } from "@/components/faq-list";
import { FinalCTA } from "@/components/final-cta";
import { AuroraBackground } from "@/components/aurora-background";
import { IsoDiagram } from "@/components/iso-diagram";

export const metadata: Metadata = {
  title: "Real User Monitoring · Traceway",
  description: "OpenTelemetry-native RUM: Web Vitals from every session, a ring buffer in the browser, real session replay, remote config over OpAMP, recording triggers, capture on demand, frontend SLOs and release regression alerts.",
  alternates: { canonical: "https://tracewayapp.com/product/rum" },
};

const TEN = [
  { id: "opentelemetry", title: "OpenTelemetry on OpAMP", line: "Standard OTLP out, standard OpAMP in. No proprietary agent." },
  { id: "metrics", title: "Metrics by default", line: "Every session measured. Nothing sampled, nothing to build." },
  { id: "ring-buffer", title: "The ring buffer", line: "The last ten minutes wait in the browser until they matter." },
  { id: "replay", title: "Real session replay", line: "The DOM, replayed. A clip for every error." },
  { id: "remote-config", title: "Remote config", line: "Change what browsers send without shipping a build." },
  { id: "triggers", title: "Recording triggers", line: "Describe a journey. The browser records it when it happens." },
  { id: "capture", title: "Capture on demand", line: "Ask one user's browser for its last ten minutes." },
  { id: "slos", title: "Frontend SLOs", line: "An error budget for INP, per release." },
  { id: "regressions", title: "Release regression alerts", line: "Told once per release, and only when it is real." },
  { id: "exemplars", title: "From p75 to the person", line: "The session, the element and the backend trace behind a slow value." },
];

const n = (i: number) => String(i).padStart(2, "0");

export default function RumPage() {
  return (
    <main className="relative">
      <section className="hero hero-product relative">
        <AuroraBackground variant="hero" />
        <div className="wrap relative z-10">
          <Chip><Monitor className="h-3 w-3 inline mr-1" />Real User Monitoring</Chip>
          <h1 className="mt-6">Measure every session.<br /><em>Upload almost nothing.</em></h1>
          <p className="hero-sub">Most RUM makes you choose: sample your users, or pay to ship every click of every healthy visit. Traceway does neither. Every page load and interaction is measured on the device, the detail waits in a ring buffer in the browser, and you decide from the dashboard what leaves it. All of it on OpenTelemetry.</p>
          <div className="hero-cta-row">
            <Link href="https://docs.tracewayapp.com/learn/rum" className="btn btn-accent">Explore RUM <ArrowRight className="h-4 w-4" /></Link>
            <Link href="https://cloud.tracewayapp.com/register" className="btn btn-ghost">Try Traceway Cloud</Link>
          </div>
        </div>
      </section>
      <div className="band-light">
        <section className="wrap py-10">
          <SectionHead align="center" eyebrow="Ten reasons" title={<>Ten things we built <em>so you can stop compromising</em></>}
            description="Not a checklist of charts. These are the ten parts of Traceway RUM we would pick a fight over." />
          <ol className="grid gap-3 sm:grid-cols-2 lg:grid-cols-5">
            {TEN.map((f, i) => (
              <li key={f.id}>
                <a href={`#${f.id}`} className="block h-full rounded-xl border border-hair-2 p-4 no-underline transition-colors hover:border-[var(--a1)]">
                  <span className="font-mono text-[11px] tracking-[0.14em]" style={{ color: "var(--a1)" }}>{n(i + 1)}</span>
                  <span className="mt-2 block font-semibold" style={{ color: "var(--fg-1)" }}>{f.title}</span>
                  <span className="mt-1 block text-sm" style={{ color: "var(--fg-2)" }}>{f.line}</span>
                </a>
              </li>
            ))}
          </ol>
        </section>

        <section id="opentelemetry" className="wrap py-10 scroll-mt-24">
          <SectionHead align="center" eyebrow="01 · OpenTelemetry on OpAMP" title={<>A standard stack, <em>all the way down</em></>}
            description="The browser SDK is OpenTelemetry: OTLP logs, spans and histograms from the Browser SIG instrumentations, and W3C trace context on every request. It is configured over OpAMP, the same protocol that manages collector fleets. Nothing in the pipeline is a format only we can read, and logs and traces can go to your own collector instead." />
          <IsoDiagram framed maxWidth={1040} src="/images/rum-architecture.png"
            alt="Traceway RUM architecture: in the browser, every number goes to a device aggregator that always uploads histograms to the metrics endpoint, feeding the Web Performance dashboard, SLOs and alerts; every event goes through a telemetry router that streams it, keeps it in a ring buffer or drops it, with a trigger engine that flushes the buffer; replay is buffered for a clip; an OpAMP client polls for collection rules." />
        </section>

        <section id="metrics" className="wrap scroll-mt-24">
          <FeatureRow reverse eyebrow="02 · Metrics by default" title={<>Every session counts. <em>No sampling, no setup.</em></>}
            description="Create a browser project and the Web Performance dashboard is already there: LCP, INP and CLS as percentiles and rating shares, with route timings, blocking time, requests and affected users. The numbers come from every visit, because each browser keeps a histogram and uploads a few bytes when the page is hidden."
            bullets={["p75 read from the merged population, never an average of averages", "Storage grows with releases, routes and devices, not with users", "No session, user or device id in a metrics payload", "Break down by release, route, device, OS and country"]}
            image={{ src: "/images/rum-dashboard.png", alt: "Web Performance dashboard populated by real browser visits in a controlled storefront demo" }} />
          <IsoDiagram framed className="pb-10" maxWidth={1040} src="/images/rum-metrics-pipeline.png"
            alt="One INP observation increments a histogram bucket on the device; one small upload when the page is hidden; payloads merge in memory at ingest and are stored per metric, dimension set and minute; reads merge the buckets, then compute percentiles, rating shares and SLOs." />
        </section>

        <section id="ring-buffer" className="wrap py-10 scroll-mt-24">
          <SectionHead align="center" eyebrow="03 · The ring buffer" title={<>The last ten minutes, <em>waiting in the browser</em></>}
            description="Most sessions are fine, and nobody will ever read their console lines. So Traceway does not upload them. Clicks, navigations, requests and console output sit in a ring buffer on the device: the last 10 minutes or 2 MB, oldest out first. Replay waits beside it. An error opens the buffer, and so can a trigger you define or a capture you start from the dashboard." />
          <IsoDiagram framed maxWidth={1040} src="/images/rum-ring-buffer.png"
            alt="The ring buffer holds the last 10 minutes or 2 MB of breadcrumbs and an 8 MB replay buffer on the device. An error, a trigger you define or a capture from the dashboard flushes up to 10 minutes of breadcrumbs and a 30 second replay clip to Traceway under one session id. What stays is saved when the page is hidden and restored by the next page of the session." />
          <BentoGrid className="mt-10">
            <BentoCell size="med" title="Healthy sessions cost almost nothing"><p>A session without a problem uploads its start, its end and its performance aggregates. No breadcrumbs, no replay.</p></BentoCell>
            <BentoCell size="med" title="The error arrives with its past"><p>Up to 10 minutes of what the user did and a 30 second replay clip are attached to the issue, source-mapped.</p></BentoCell>
            <BentoCell size="med" title="It survives page loads"><p>The buffer is saved when a page is hidden and restored by the next one, so a checkout that fails on page three still has pages one and two.</p></BentoCell>
          </BentoGrid>
        </section>

        <section id="replay" className="wrap scroll-mt-24">
          <FeatureRow eyebrow="04 · Real session replay" title={<>Not a screenshot. <em>The page, replayed.</em></>}
            description="Replay records the DOM and what changed in it, so you watch the real page with the real layout at any speed, next to the console lines, requests and clicks of the same session. Every error gets a clip of the moments before it. Whole sessions are recorded for the cohort you choose, not for everyone."
            bullets={["Inputs masked by default", "A 30 second clip attached to each error, out of the buffer", "Timeline of logs, actions and requests beside the player", "Session replay for Flutter apps too"]}
            image={{ src: "/images/session-replay-viewer.png", alt: "Session replay player showing a recorded storefront visit with playback speed controls" }}
            cta={<Link href="/product/session-replay" className="btn btn-ghost">Session replay <ArrowRight className="h-4 w-4" /></Link>} />
        </section>

        <section id="remote-config" className="wrap scroll-mt-24">
          <FeatureRow reverse eyebrow="05 · Remote config" title={<>Change what browsers send. <em>Ship nothing.</em></>}
            description="Each signal is off, buffered on the device or streamed, and you set that from the dashboard per group of clients: one release, one browser, one tenant, a tenth of traffic. Browsers pick the rules up over OpAMP at their next poll. Turn replay on for the release you are worried about, quiet a noisy console, or stop collection entirely for a broken build."
            bullets={["Target by release, browser, user or any attribute, with a sticky percentage rollout", "Errors, network, console, clicks, vitals, replay and metrics, each on its own switch", "Remote rules only restrict what your init() allows", "Preview the exact settings a given client receives", "A kill switch that needs no deploy"]}
            image={{ src: "/images/rum-remote-config.png", alt: "Data Collection rule editor with a buffer, stream or inherit setting for each browser signal" }}
            cta={<Link href="https://docs.tracewayapp.com/learn/remote-config" className="btn btn-ghost">Remote config <ArrowRight className="h-4 w-4" /></Link>} />
        </section>

        <section id="triggers" className="wrap scroll-mt-24">
          <FeatureRow eyebrow="06 · Recording triggers" title={<>Describe the journey. <em>The browser does the rest.</em></>}
            description="Build a sequence of clicks, hovers, navigations and log lines in a visual editor: opened checkout, then three failed requests in a minute. The sequence is matched on the device, so nothing is uploaded while it has not happened. When it completes, the browser sends the history that led up to it and a replay."
            bullets={["Ordered steps and counts within a time window, no expression language", "Bounded history, cooldowns and per-session limits", "Delivered by remote config, so a new trigger needs no release"]}
            image={{ src: "/images/rum-sequence-builder.png", alt: "Traceway visual editor configuring click, navigation and log steps for a replay capture" }}
            cta={<Link href="https://docs.tracewayapp.com/learn/remote-config/trigger-sequences" className="btn btn-ghost">Build a recording trigger <ArrowRight className="h-4 w-4" /></Link>} />
          <FeatureRow reverse eyebrow="Every recording has a reason" title={<>Find the sessions <em>that followed your rule</em></>}
            description="A session can match more than one journey. Its trigger tags say which sequences asked for the recording, and each tag opens a shareable filter. The reasons survive rule edits and deletion."
            image={{ src: "/images/rum-sequence-sessions.png", alt: "Session list showing multiple recording trigger tags with readable action sequences" }} />
        </section>

        <section id="capture" className="wrap scroll-mt-24">
          <FeatureRow eyebrow="07 · Capture on demand" title={<>&ldquo;It&apos;s broken for me.&rdquo; <em>Go and look.</em></>}
            description="A customer reports something you cannot reproduce. Start a capture for their user, device or session: at its next poll their browser uploads what it had buffered, up to the last ten minutes, and streams everything until the capture ends. With replay if you want it. One click from the session or the issue page."
            bullets={["Target one user, device, session or attribute", "From a few minutes up to 24 hours, cancel any time", "Brings back signals that rules had switched off, within your init() limits"]}
            image={{ src: "/images/rum-capture.png", alt: "New Capture dialog opened from a session page, targeting one device for an hour with the last 10 minutes and session replay" }} />
        </section>

        <section id="slos" className="wrap scroll-mt-24">
          <FeatureRow reverse eyebrow="08 · Frontend SLOs" title={<>Give the experience <em>an error budget</em></>}
            description="Define good as an interaction at or below 200 ms, then choose the share your product promises to meet. Every observation counts toward the objective. Split by release to see which rollout is spending the budget, and page someone on burn rate when it needs attention."
            bullets={["Objectives over observed events, rather than averages of averages", "One budget per release, device or country from a single form", "Crash-free sessions as an objective of its own", "Burn-rate alerts into the same on-call as your backend"]}
            image={{ src: "/images/rum-slos-table.png", alt: "Frontend interaction SLOs: demo release 0.1.0 meets its target while intentionally slowed release 0.2.0 breaches it" }}
            cta={<Link href="/product/slos" className="btn btn-ghost">Explore frontend SLOs <ArrowRight className="h-4 w-4" /></Link>} />
        </section>

        <section id="regressions" className="wrap scroll-mt-24">
          <FeatureRow eyebrow="09 · Release regression alerts" title={<>A release can be fast overall <em>and slow on phones</em></>}
            description="One rule compares the newest release with the one before it and tells you once, when the difference is real. The earlier release is weighted to the new one's mix of devices and operating systems, so a rollout that reached slower phones first is not reported as a code regression. A percentile has to be worse by more than chance explains."
            bullets={["Fires once per release, not once per evaluation", "Adjusted for device mix, or compared inside each segment", "Waits for enough observations before judging"]}
            image={{ src: "/images/rum-regression-alert.png", alt: "Alert rule of type Release Regression on p75 INP, compared across the whole release adjusted for device mix" }} />
        </section>

        <section id="exemplars" className="wrap scroll-mt-24">
          <FeatureRow reverse eyebrow="10 · From p75 to the person" title={<>A slow number <em>with a name on it</em></>}
            description="A percentile tells you something is slow. Click Slow sessions on any chart for the worst values behind it, each with its page and the element the browser blamed. Open the session, watch what the user did, then follow the request into the backend spans it caused: the browser sends the W3C traceparent header, so it is one trace."
            bullets={["The element behind a poor LCP, INP or CLS", "The script that held the main thread", "Browser and backend spans in one waterfall"]}
            image={{ src: "/images/rum-slow-sessions.png", alt: "Web Performance dashboard with the slow sessions list of INP open, showing the worst values and the button element behind each" }}
            cta={<Link href="/product/traces" className="btn btn-ghost">Distributed traces <ArrowRight className="h-4 w-4" /></Link>} />
          <FeatureRow eyebrow="One click later" title={<>The session <em>behind the value</em></>}
            description="The exemplar opens the session it came from: attributes, timeline, errors and, when one was recorded, the replay."
            image={{ src: "/images/rum-slow-session.png", alt: "Session investigation opened from a poor INP observation in the frontend dashboard" }}
            cta={<Link href="https://docs.tracewayapp.com/learn/sessions" className="btn btn-ghost">Sessions and replay <ArrowRight className="h-4 w-4" /></Link>} />
        </section>

        <section className="wrap py-10">
          <SectionHead eyebrow="And the rest" title={<>Everything else a frontend team <em>asks of RUM</em></>} />
          <BentoGrid>
            <BentoCell size="med" icon={Activity} title="Web Vitals, per page and per view" iconColor="var(--a1)"><p>LCP, INP, CLS, FCP and TTFB, with INP and CLS measured again for every single-page-app route change.</p></BentoCell>
            <BentoCell size="med" icon={Route} title="Routes, long tasks, requests" iconColor="var(--a2)"><p>Route-change timing, main-thread blocking time, and request duration and failure rate by host.</p></BentoCell>
            <BentoCell size="med" icon={Users} title="Users affected" iconColor="var(--ok)"><p>Browser profiles hit by an error, a poor vital, a rage click or a failed request. Counted on the device, so no id is sent.</p></BentoCell>
            <BentoCell size="med" icon={Bug} title="Errors with their context" iconColor="var(--a1)"><p>Source-mapped issues with the clicks, requests and console lines that led up to them.</p></BentoCell>
            <BentoCell size="med" icon={Ruler} title="Your own metrics" iconColor="var(--a2)"><p>metric() and count() for the numbers only your app knows, with your own dimensions such as plan or experiment.</p></BentoCell>
            <BentoCell size="med" icon={ScanSearch} title="Segments that matter" iconColor="var(--ok)"><p>Release, route, browser, OS, form factor, device class, network type and country.</p></BentoCell>
            <BentoCell size="med" icon={EyeOff} title="Private by construction" iconColor="var(--a1)"><p>Metrics carry no identifiers, replay masks inputs, and a healthy session never uploads its clicks.</p></BentoCell>
            <BentoCell size="med" icon={BellRing} title="Alerts that reach someone" iconColor="var(--a2)"><p>Thresholds on any percentile or good share, routed to Slack, email or an on-call escalation policy.</p></BentoCell>
            <BentoCell size="med" icon={Server} title="Cloud or your own server" iconColor="var(--ok)"><p>The same RUM on Traceway Cloud and on a self-hosted server, down to a single container.</p></BentoCell>
          </BentoGrid>
          <p className="mt-8 text-sm text-muted-foreground">Screenshots show controlled storefront and user-directory demos with real browser telemetry. Release 0.2.0 is intentionally slowed to demonstrate regressions; these are not customer benchmarks.</p>
        </section>
      </div>
      <FinalCTA title={<>Ship a frontend <em>you can measure.</em></>} description="Frontend metrics, objectives, alerts and replay in the same investigation workflow, on OpenTelemetry. Available with Cloud or a self-hosted Traceway server." primary={{ label: "Read the RUM guide", href: "https://docs.tracewayapp.com/learn/rum" }} />
      <section className="wrap pt-10 pb-24"><div className="max-w-3xl mx-auto">
        <SectionHead align="center" eyebrow="FAQ" title="RUM, metrics and replay" />
        <FaqList items={[
          { q: "Is RUM the same as session replay?", a: "Replay shows a recorded experience. RUM also measures performance across the observed population, compares releases and devices, and supports objectives and alerts. Replay is one investigation tool within RUM." },
          { q: "What is the ring buffer?", a: "A bounded store in the browser that keeps the newest clicks, navigations, requests and console lines: the last 10 minutes or 2 MB. Nothing in it is uploaded while a session is healthy. An error, a trigger you define or a capture you start uploads the window that led up to the moment, with a replay clip. It is saved when a page is hidden and restored by the next page of the same session." },
          { q: "What does OpAMP have to do with a browser?", a: "OpAMP is the OpenTelemetry protocol for managing agents remotely. The browser SDK uses its plain HTTP transport to ask Traceway which collection rules apply to it, by default every five minutes. There is no push: a change takes effect at a browser's next poll." },
          { q: "Does monitoring slow the page down?", a: "The SDK is built to stay out of the way: performance numbers are a bucket increment, the buffer is written to storage once when the page is hidden instead of on a timer, replay is only loaded when it is not switched off, and error storms are rate limited on the device. A session has a ceiling on what it may upload." },
          { q: "Do metrics require recording every session?", a: "No. Browser performance metrics are aggregated on the device and uploaded as distributions or counters. Replay collection has separate controls. A metric can exist without a replay, and a slow-session exemplar does not guarantee a recording." },
          { q: "Which SDK version do I need?", a: "The aggregated browser-metrics path requires SDK 2.1 or later and a compatible server. Use the installation guide for the current release channel; the 2.x browser documentation currently uses the beta tag. Existing 2.0 performance events can also populate metrics on the server." },
          { q: "What about privacy and sampling?", a: "Use normalized routes and low-cardinality dimensions, never user IDs or emails as metric labels. Configure replay masking and collection for your application. Sampled totals and histogram percentiles are estimates; blocked telemetry and unsupported browser APIs affect coverage." },
        ]} />
      </div></section>
    </main>
  );
}
