import Link from "next/link";
import type { Metadata } from "next";
import { ArrowRight, Monitor, Network, FileJson, Server, EyeOff, Keyboard, Gauge } from "lucide-react";
import { Chip } from "@/components/chip";
import { SectionHead } from "@/components/section-head";
import { FeatureRow } from "@/components/feature-row";
import { BentoGrid, BentoCell } from "@/components/bento-grid";
import { FaqList } from "@/components/faq-list";
import { FinalCTA } from "@/components/final-cta";
import { AuroraBackground } from "@/components/aurora-background";

export const metadata: Metadata = {
  title: "Real User Monitoring · Traceway",
  description:
    "See how fast your site is for real visitors, which release made it slower, and what a user did before an error. Every visit is measured, and the details stay in the browser until something goes wrong. Built on OpenTelemetry.",
  alternates: { canonical: "https://tracewayapp.com/product/rum" },
};

const QUESTIONS = [
  {
    q: "Is the site fast?",
    a: "How long pages take to load, how quickly they react to a click, and whether the layout jumps around. Measured on your visitors' own devices and networks, not in a lab.",
  },
  {
    q: "Did the last release make it worse?",
    a: "Compare releases side by side, and get an alert when a new one is slower than the one before it.",
  },
  {
    q: "Who is affected?",
    a: "Split any number by page, browser, device, country or release, and see how many users hit an error or a slow page.",
  },
  {
    q: "What happened to this user?",
    a: "Open a single visit to see its clicks, requests and errors in order, and watch a replay of what was on the screen.",
  },
];

const DIFFERENCES = [
  {
    title: "Speed is measured for every visit",
    body: "Each browser adds up its own page timings and sends one small summary instead of a record for every event. Nobody is left out of the numbers. Visits to the same page on the same release are stored together, so storage depends on how many pages and releases you have, not on how many visitors.",
  },
  {
    title: "Details stay in the browser until they are needed",
    body: "The last 10 minutes of clicks, requests, console output and page recording are kept in the visitor's browser. They are uploaded when an error happens, when a trigger you set up matches, or when you ask for one user's data. A visit with no problems never uploads them.",
  },
];

const COMPARISON = [
  {
    q: "Whose page speed is in the numbers",
    usual: "Only the visits you sample.",
    traceway: "Every visit.",
  },
  {
    q: "What a visit with no problems uploads",
    usual: "Every click and request, and often the whole recording.",
    traceway: "A small summary of its timings, plus when it started and ended.",
  },
  {
    q: "What you have when an error happens",
    usual: "The full story only if that visit was one of the sampled ones.",
    traceway: "The error, up to 10 minutes of what led to it, and a 30 second replay.",
  },
  {
    q: "A customer reports a bug you cannot reproduce",
    usual: "Nothing to look at unless their visit happened to be recorded.",
    traceway: "Ask their browser for its recent history, then watch what happens next.",
  },
  {
    q: "Changing what is collected",
    usual: "Usually a code change and a new release.",
    traceway: "A setting in the dashboard, picked up within minutes.",
  },
  {
    q: "The data format",
    usual: "Usually one that only the vendor's own tools can read.",
    traceway: "Standard OpenTelemetry. Open source, on our cloud or your own server.",
  },
];

const COMPARISON_COLUMNS = "md:grid md:grid-cols-[0.9fr_1fr_1fr]";

export default function RumPage() {
  return (
    <main className="relative">
      <section className="hero hero-product relative">
        <AuroraBackground variant="hero" />
        <div className="wrap relative z-10">
          <Chip><Monitor className="h-3 w-3 inline mr-1" />Real User Monitoring</Chip>
          <h1 className="mt-6">Know when your site is slow or broken <em>for real users</em></h1>
          <p className="hero-sub">
            Traceway measures page speed and errors in your visitors&apos; browsers. You see which pages are slow,
            which release made them slower, how many people were affected, and what a user did right before
            something broke. Every visit is measured, not a sample.
          </p>
          <div className="hero-cta-row">
            <Link href="https://docs.tracewayapp.com/learn/rum" className="btn btn-accent">Read the RUM guide <ArrowRight className="h-4 w-4" /></Link>
            <Link href="https://cloud.tracewayapp.com/register" className="btn btn-ghost">Try Traceway Cloud</Link>
          </div>
        </div>
      </section>

      <div className="band-light">
        <section className="wrap py-10">
          <SectionHead align="center" eyebrow="What it tells you" title="Four questions it answers" />
          <ul className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
            {QUESTIONS.map((item) => (
              <li key={item.q} className="rounded-xl border border-hair-2 p-5">
                <h3 className="text-base font-semibold text-fg-0">{item.q}</h3>
                <p className="mt-2 text-sm text-fg-2">{item.a}</p>
              </li>
            ))}
          </ul>
        </section>

        <section id="how-it-differs" className="wrap py-10 scroll-mt-24">
          <SectionHead
            align="center"
            eyebrow="Compared with other tools"
            title="How it differs from other RUM tools"
            description="Most RUM tools upload everything each visitor does: every click, every request, often a full recording. That is a lot of data, so they charge by the session, and teams end up watching only a share of their visitors. Traceway works differently in two ways."
          />
          <div className="mx-auto grid max-w-[1040px] gap-4 md:grid-cols-2">
            {DIFFERENCES.map((item, i) => (
              <div key={item.title} className="rounded-xl border border-hair-2 p-6">
                <span className="font-mono text-[11px] tracking-[0.14em] text-fg-3">{i + 1}</span>
                <h3 className="mt-2 text-lg font-semibold text-fg-0">{item.title}</h3>
                <p className="mt-2 text-fg-2">{item.body}</p>
              </div>
            ))}
          </div>

          <div className="mx-auto mt-10 max-w-[1040px] overflow-hidden rounded-xl border border-hair-2 text-[15px]">
            <div aria-hidden className={`hidden ${COMPARISON_COLUMNS} border-b border-hair-2 font-semibold text-fg-0`}>
              <div className="p-4" />
              <div className="p-4">The usual approach</div>
              <div className="p-4">Traceway</div>
            </div>
            {COMPARISON.map((row, i) => (
              <div key={row.q} className={`${COMPARISON_COLUMNS} ${i > 0 ? "border-t border-hair-2" : ""}`}>
                <div className="px-4 pt-4 font-semibold text-fg-0 md:pb-4">{row.q}</div>
                <div className="px-4 pt-2 text-fg-2 md:py-4">
                  <span className="font-medium md:sr-only">The usual approach: </span>
                  {row.usual}
                </div>
                <div className="px-4 pt-2 pb-4 text-fg-0 md:py-4">
                  <span className="font-medium md:sr-only">Traceway: </span>
                  {row.traceway}
                </div>
              </div>
            ))}
          </div>
          <p className="mx-auto mt-4 max-w-[1040px] text-sm text-fg-3">
            Tools differ. The middle column describes the common approach of uploading everything and sampling to keep
            the cost down.{" "}
            <Link href="https://docs.tracewayapp.com/learn/rum#how-it-works" className="underline">
              How Traceway RUM works in detail
            </Link>
          </p>
        </section>

        <section id="metrics" className="wrap scroll-mt-24">
          <FeatureRow
            reverse
            eyebrow="Page speed"
            title={<>See how fast your pages are <em>for every visitor</em></>}
            description="Create a browser project and the Web Performance dashboard is already set up. It shows the three Core Web Vitals: how long the main content takes to appear (LCP), how fast the page reacts to a click or tap (INP), and how much the layout jumps around (CLS). You also get page load times, slow requests and how many users were affected."
            bullets={[
              "Every visit is counted, with no sampling by default",
              "Split any chart by page, release, browser, device or country",
              "See what share of visits were good, need improvement or poor",
              "Track your own numbers too, such as how long checkout takes to render",
            ]}
            image={{ src: "/images/rum-dashboard.png", alt: "Web Performance dashboard populated by real browser visits in a controlled storefront demo" }}
          />
        </section>

        <section id="regressions" className="wrap scroll-mt-24">
          <FeatureRow
            eyebrow="Releases"
            title={<>Know when a release <em>makes the site slower</em></>}
            description="Traceway compares each new release with the one before it and tells you once if it got slower. The comparison is fair. If the new release happened to reach more old phones first, that is taken into account instead of being reported as a problem in your code. It also waits until enough visits have come in to be sure the difference is real."
            bullets={[
              "One alert per release, not one every few minutes",
              "Compare the release as a whole, or phones and desktops separately",
              "Delivered to Slack, email or whoever is on call",
            ]}
            image={{ src: "/images/rum-regression-alert.png", alt: "Alert rule of type Release Regression on p75 INP, compared across the whole release adjusted for device mix" }}
          />
        </section>

        <section id="slos" className="wrap scroll-mt-24">
          <FeatureRow
            reverse
            eyebrow="Targets"
            title={<>Set a speed target <em>and see if you are meeting it</em></>}
            description="Decide what good means, for example a page that reacts to a click within 200 ms, and how often it should happen, say 95% of the time. Traceway tracks it over 7, 30 or 90 days and shows how much room is left before the target is missed. Backend teams call this a service level objective, or SLO."
            bullets={[
              "A separate target per release, so you can see which one is causing the misses",
              "Targets for page speed, a single page, or sessions that ended without an error",
              "An alert when you are on course to miss the target",
            ]}
            image={{ src: "/images/rum-slos-table.png", alt: "Frontend interaction SLOs: demo release 0.1.0 meets its target while intentionally slowed release 0.2.0 breaches it" }}
            cta={<Link href="/product/slos" className="btn btn-ghost">More about SLOs <ArrowRight className="h-4 w-4" /></Link>}
          />
        </section>

        <section id="slow-sessions" className="wrap scroll-mt-24">
          <FeatureRow
            eyebrow="Slow visits"
            title={<>Find out <em>why a page was slow</em></>}
            description="A chart tells you something is slow, not why. Click Slow sessions on any chart to see the slowest visits behind it, each with its page and the element that was slow, such as a button that took too long to respond. Open one to see what the user did, step by step. If your backend sends its traces to Traceway too, each request links to the work the server did for it."
            bullets={[
              "The element behind a slow load, a slow click or a layout jump",
              "The script that kept the page busy",
              "Browser and backend on one timeline",
            ]}
            image={{ src: "/images/rum-slow-sessions.png", alt: "Web Performance dashboard with the slow sessions list of INP open, showing the worst values and the button element behind each" }}
            cta={<Link href="/product/traces" className="btn btn-ghost">More about traces <ArrowRight className="h-4 w-4" /></Link>}
          />
        </section>

        <section id="replay" className="wrap scroll-mt-24">
          <FeatureRow
            reverse
            eyebrow="Errors and replay"
            title={<>Every error comes with <em>what the user did before it</em></>}
            description="While a visit is going well, the last 10 minutes of clicks, page changes, requests and console output stay in the browser, along with a recording of the page. When an error happens, that history is uploaded with it. You get the error, the steps that led to it, and a 30 second replay of what the user saw."
            bullets={[
              "Replay rebuilds the real page, and you can watch it at any speed",
              "Anything typed into a form field is hidden by default",
              "The history carries over from page to page, so an error on step three of checkout still shows steps one and two",
              "Stack traces point to your original code once source maps are uploaded",
            ]}
            image={{ src: "/images/session-replay-viewer.png", alt: "Session replay player showing a recorded storefront visit with playback speed controls" }}
            cta={<Link href="https://docs.tracewayapp.com/learn/sessions" className="btn btn-ghost">Sessions and replay <ArrowRight className="h-4 w-4" /></Link>}
          />
        </section>

        <section id="capture" className="wrap scroll-mt-24">
          <FeatureRow
            eyebrow="Support cases"
            title={<>Look into a problem <em>only one customer has</em></>}
            description="A customer reports something you cannot reproduce. Start a capture for that user from the dashboard. The next time their browser checks in, it uploads what it kept from the last 10 minutes and keeps sending until the capture ends, with a replay if you want one. You look at their real visit instead of guessing from a description."
            bullets={[
              "Choose one user, one device or one open session",
              "Runs from a minute up to 24 hours, and you can stop it early",
              "Browsers check in every five minutes by default, so it can take a few minutes to start",
            ]}
            image={{ src: "/images/rum-capture.png", alt: "New Capture dialog opened from a session page, targeting one device for an hour with the last 10 minutes and session replay" }}
          />
        </section>

        <section id="triggers" className="wrap scroll-mt-24">
          <FeatureRow
            reverse
            eyebrow="Recording triggers"
            title={<>Record the situations <em>you care about</em></>}
            description="Not every problem throws an error: a payment fails, the page freezes, someone clicks Pay and then leaves. Describe the situation as a few steps in a visual editor, for example opened checkout, then three failed requests within a minute. The browser watches for it, and when it happens it uploads the history and a replay. Until then, nothing is sent."
            bullets={[
              "Steps can be clicks, hovers, page changes or log lines",
              "No code change: new triggers reach browsers on their own",
              "Each recorded session shows which trigger asked for it, and you can filter by trigger",
            ]}
            image={{ src: "/images/rum-sequence-builder.png", alt: "Traceway visual editor configuring click, navigation and log steps for a replay capture" }}
            cta={<Link href="https://docs.tracewayapp.com/learn/remote-config/trigger-sequences" className="btn btn-ghost">How recording triggers work <ArrowRight className="h-4 w-4" /></Link>}
          />
        </section>

        <section id="remote-config" className="wrap scroll-mt-24">
          <FeatureRow
            eyebrow="Data collection"
            title={<>Change what is collected <em>without a release</em></>}
            description="For each kind of data (errors, requests, console output, clicks, replay) you pick one of three settings: send it right away, keep it in the browser until it is needed, or do not collect it. You set this in the dashboard, for everyone or for one release, one browser or a percentage of users. Browsers pick up the change within a few minutes."
            bullets={[
              "Turn on full replay for the release you are worried about",
              "Switch collection off for a broken build without deploying",
              "The dashboard can only collect less than your code allows, never more",
              "Preview the settings a given user would receive",
            ]}
            image={{ src: "/images/rum-remote-config.png", alt: "Data Collection rule editor with a buffer, stream or inherit setting for each browser signal" }}
            cta={<Link href="https://docs.tracewayapp.com/learn/remote-config" className="btn btn-ghost">How remote settings work <ArrowRight className="h-4 w-4" /></Link>}
          />
        </section>

        <section id="opentelemetry" className="wrap py-10 scroll-mt-24">
          <SectionHead eyebrow="Good to know" title="Open standards, privacy and limits" />
          <BentoGrid>
            <BentoCell size="med" icon={Network} title="Browser and backend in one trace" iconColor="var(--a1)">
              <p>When your backend also reports to Traceway, a slow click can be followed from the browser down to the database query behind it.</p>
            </BentoCell>
            <BentoCell size="med" icon={FileJson} title="A standard data format" iconColor="var(--a2)">
              <p>The browser SDK sends OpenTelemetry logs, traces and metrics. Nothing is in a format only we can read, and logs and traces can go to your own collector instead.</p>
            </BentoCell>
            <BentoCell size="med" icon={Server} title="Cloud or your own server" iconColor="var(--ok)">
              <p>Traceway is open source under the MIT license. RUM works the same on Traceway Cloud and on a server you run yourself.</p>
            </BentoCell>
            <BentoCell size="med" icon={EyeOff} title="No user ids in the speed numbers" iconColor="var(--a1)">
              <p>Performance data carries no user, session or device id and no full URL, only the page pattern, such as /orders/:id.</p>
            </BentoCell>
            <BentoCell size="med" icon={Keyboard} title="Typed input is hidden" iconColor="var(--a2)">
              <p>Replay hides what users type into form fields by default, and a visit with no problems never uploads its clicks or its recording.</p>
            </BentoCell>
            <BentoCell size="med" icon={Gauge} title="A limit on every browser" iconColor="var(--ok)">
              <p>Repeated errors are rate limited and each session has a cap on how much it may upload, so monitoring cannot flood a user&apos;s connection.</p>
            </BentoCell>
          </BentoGrid>
          <p className="mt-8 text-sm text-muted-foreground">
            The screenshots show a demo shop and a demo user directory with real browser traffic. Release 0.2.0 was
            slowed down on purpose to show what a regression looks like. These are not customer benchmarks.
          </p>
        </section>
      </div>

      <FinalCTA
        title={<>Try it <em>on your own site</em></>}
        description="Create a browser project, add the SDK, and the dashboard fills in as visitors arrive. Free to start on Traceway Cloud, or run it on your own server."
        primary={{ label: "Read the RUM guide", href: "https://docs.tracewayapp.com/learn/rum" }}
        secondary={{ label: "Try Traceway Cloud", href: "https://cloud.tracewayapp.com/register" }}
      />

      <section className="wrap pt-10 pb-24">
        <div className="max-w-3xl mx-auto">
          <SectionHead align="center" eyebrow="FAQ" title="Questions about real user monitoring" />
          <FaqList
            items={[
              {
                q: "What is real user monitoring?",
                a: "It measures what happens in your visitors' browsers: how long pages take to load, how quickly they react, and which errors occur. Server monitoring cannot see this. A request can succeed on the server while the page still takes five seconds to appear on a phone.",
              },
              {
                q: "How is this different from session replay?",
                a: "Session replay shows one visit. RUM also measures all visits together, so you can tell whether a problem affects one person or a third of your users, and which release caused it. In Traceway, replay is one of the tools you use once the numbers have shown you where to look.",
              },
              {
                q: "Do you record every session?",
                a: "No. Every visit is measured for speed and errors, but the recording stays in the browser. It is uploaded when an error happens, when one of your recording triggers matches, or when you start a capture for a user. You can turn on full recording for a group of users if you want it.",
              },
              {
                q: "What is the ring buffer?",
                a: "It is the storage in the browser that holds the recent history of a visit: the last 10 minutes or 2 MB of clicks, page changes, requests and console output, whichever limit is reached first. When it is full, the oldest entries are dropped to make room for new ones, which is why it is called a ring. Nothing in it is uploaded while a visit is going well. An error, a recording trigger or a capture uploads what it holds. It is saved when a page is hidden and restored by the next page of the same visit.",
              },
              {
                q: "Will it slow my site down?",
                a: "It adds some work in the browser, like any monitoring script, and it is built to keep that small. Timings are added to a running total instead of being sent one by one. The saved history is written to storage once, when the page is hidden, never on a timer. The replay recorder is not loaded at all if you switch replay off.",
              },
              {
                q: "How accurate are the numbers?",
                a: "Timings are grouped into ranges about 9% wide before they are sent, so a value such as the 75th percentile is accurate to within a few percent. That is precise enough to compare pages and releases. Visits where the browser blocks monitoring are not counted.",
              },
              {
                q: "What is OpAMP, and how do settings reach the browser?",
                a: "OpAMP is the OpenTelemetry protocol for managing agents remotely. It is normally used to configure fleets of collectors on servers. Traceway's browser SDK uses the same protocol to ask which collection settings apply to it, every five minutes by default. There is no push connection, so a new setting or a capture takes effect the next time a browser asks.",
              },
              {
                q: "What data about my users leaves the browser?",
                a: "Performance numbers contain no user, session or device id and no full URL. The detailed history and the replay are uploaded only in the cases above, and replay hides typed form input by default. You should still review what your pages show on screen before turning replay on.",
              },
              {
                q: "Which frameworks and SDK version does it need?",
                a: "React, Vue, Svelte and jQuery have their own packages, and any other site can use the plain JavaScript package. The features on this page need version 2 of the browser SDK, which is currently in beta, and the speed numbers need 2.1 or later. The installation guide has the current install command.",
              },
            ]}
          />
        </div>
      </section>
    </main>
  );
}
