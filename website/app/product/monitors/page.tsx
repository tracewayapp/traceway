import Link from "next/link";
import type { Metadata } from "next";
import { ArrowRight, Radar } from "lucide-react";

import { Chip } from "@/components/chip";
import { SectionHead } from "@/components/section-head";
import { FeatureRow } from "@/components/feature-row";
import { FaqList } from "@/components/faq-list";
import { FinalCTA } from "@/components/final-cta";
import { AuroraBackground } from "@/components/aurora-background";

export const metadata: Metadata = {
  title: "Monitors · Traceway",
  description:
    "Synthetic uptime monitoring built into your observability stack. HTTP, TCP, and real-browser Playwright checks, flap damping, failure screenshots, public status pages, and paging that resolves itself on recovery.",
};

export default function MonitorsPage() {
  return (
    <main className="relative">
      <section className="hero hero-product relative">
        <AuroraBackground variant="hero" />
        <div className="wrap relative z-10">
          <Chip variant="ok">
            <Radar className="h-3 w-3 inline mr-1" />
            Uptime Monitors &amp; Status Pages
          </Chip>
          <h1 className="mt-6">
            Telemetry sees inside. <em>Monitors check from outside.</em>
          </h1>
          <p className="hero-sub">
            Traces and logs only exist while requests arrive. A monitor keeps
            probing on a schedule, an HTTP request, a TCP connect, or a full
            Playwright browser flow, so you find out your site is down before a
            user tells you. In the same tool that already holds the stack
            trace.
          </p>
          <div className="hero-cta-row">
            <Link
              href="https://docs.tracewayapp.com/learn/monitors"
              className="btn btn-accent"
            >
              Read the Docs <ArrowRight className="h-4 w-4" />
            </Link>
            <Link
              href="https://cloud.tracewayapp.com/register"
              className="btn btn-ghost"
            >
              Try Traceway Cloud
            </Link>
          </div>
        </div>
      </section>

      {/* WHITE BAND: feature sections render on white */}
      <div className="band-light">
        <section className="wrap">
          <FeatureRow
            eyebrow="Three kinds of probe"
            title={
              <>
                An HTTP request, a TCP connect, <em>or a real browser</em>
              </>
            }
            description="HTTP checks probe like Postman: method, headers, auth, body, and assertions on status codes, response content, response time, and days left on the TLS certificate. TCP checks cover databases, SMTP, and anything that is not HTTP. Every monitor has an interval down to 30 seconds, a timeout, and a failure threshold, so one flaky probe never wakes anyone at 3am."
            bullets={[
              "Assert on status, body substring or regex, latency, and TLS expiry",
              "TCP with optional TLS, send payload, and expected response",
              "Down only after N consecutive failures, flap damping built in",
              "Uptime and average latency per monitor, at a glance",
            ]}
            image={{
              src: "/images/monitors-list.png",
              alt: "The Traceway monitors list with HTTP, TCP, and browser checks and their uptime",
            }}
          />
        </section>

        <section className="wrap">
          <FeatureRow
            reverse
            eyebrow="Browser checks"
            title={
              <>
                Your uptime probe is <em>a Playwright test</em>
              </>
            }
            description="Write a full @playwright/test spec in the editor on the monitor itself, and it runs in headless Chromium on a schedule. A login flow or a checkout journey becomes your availability check, so 'up' means the thing your users actually do still works, not just that the load balancer answers 200."
            bullets={[
              "Real @playwright/test specs, syntax highlighted in the dialog",
              "Per-check environment variables; scripts never see server secrets",
              "Runs embedded in the :browser image, or on remote runner machines",
              "Runners connect outbound-only, no inbound ports to open",
            ]}
            image={{
              src: "/images/monitors-editor.png",
              alt: "Editing a Playwright browser check in Traceway",
            }}
          />
        </section>

        <section className="wrap">
          <FeatureRow
            eyebrow="When it breaks"
            title={
              <>
                Down comes with <em>evidence</em>
              </>
            }
            description="A failed browser run automatically stores the failure screenshot and the Playwright output, both one click from the run history. The monitor page shows availability, a latency chart, every probe with its error, and the incident timeline, so the person who responds starts with the answer instead of a blank terminal."
            bullets={[
              "Failure screenshot and full Playwright logs per failed run",
              "Per-probe latency, status code, and error message",
              "Incidents open and resolve themselves from state transitions",
              "A probe past its window is recorded as missed, never run late",
            ]}
            image={{
              src: "/images/monitors-detail.png",
              alt: "A monitor detail page with availability, latency chart, and failed runs with screenshots and logs",
            }}
          />
        </section>

        <section className="wrap">
          <FeatureRow
            reverse
            eyebrow="Status pages"
            title={
              <>
                Tell your users <em>before they ask</em>
              </>
            }
            description="Publish the uptime of selected monitors on a public status page: 90 days of per-day availability, incident history, your name, your logo, no login required. Serve it from your own domain with a CNAME. Latency numbers stay internal; the public page only shows availability."
            bullets={[
              "Public at /status/your-slug, or on your own custom domain",
              "90 days of per-day uptime bars with incident history",
              "Branding: title, description, and an uploaded logo",
              "Refreshes itself while your users watch",
            ]}
            image={{
              src: "/images/monitors-status-page.png",
              alt: "A public Traceway status page with 90 days of uptime bars and an active incident",
            }}
          />
        </section>

        <section className="wrap">
          <FeatureRow
            eyebrow="Wired into alerting"
            title={
              <>
                Down pages a human, recovery <em>cleans up after itself</em>
              </>
            }
            description="Add a Monitor Down rule and every monitor in the project is covered: a message to Slack, or a real page through your on-call escalation policy. Pages deduplicate per monitor, re-fires never restart the escalation clock, and when the check recovers the page resolves itself, so a blip at night that recovers on its own never needs a human."
            bullets={[
              "One rule covers every monitor in the project",
              "Escalation channels open an on-call page instead of a message",
              "Recovery auto-resolves the page and sends the all-clear",
              "The alert carries the monitor name and the last probe error",
            ]}
            image={{
              src: "/images/oncall-pages.png",
              alt: "The Traceway on-call incident queue fed by monitor state changes",
            }}
          />
        </section>
      </div>

      <FinalCTA
        title={
          <>
            Stop paying for <em>a separate uptime tool</em>
          </>
        }
        description="Monitors, status pages, alerting, and on-call in the same tool that holds your traces, logs, and errors. Open source if you self-host."
        primary={{
          label: "Read the Monitors docs",
          href: "https://docs.tracewayapp.com/learn/monitors",
        }}
        secondary={{
          label: "Start Free",
          href: "https://cloud.tracewayapp.com/register",
        }}
      />

      <section className="wrap pt-10 pb-24">
        <div className="max-w-3xl mx-auto">
          <SectionHead
            align="center"
            eyebrow="FAQ"
            title="Questions about monitors"
          />
          <div className="mt-4">
            <FaqList
              items={[
                {
                  q: "Do I still need UptimeRobot or Pingdom?",
                  a: "Not for the checks themselves. Traceway covers HTTP, TCP, and real-browser probes, flap damping, incident history, public status pages with custom domains, and alerting that can page a rotating on-call schedule. The difference is that the monitor, the alert, the trace, and the stack trace live in one place, so 'the site is down' and 'here is why' are the same screen.",
                },
                {
                  q: "Where do browser checks run?",
                  a: (
                    <>
                      <p>
                        Wherever you decide. The <code>:browser</code> Docker
                        image runs them inside the container with Chromium
                        baked in. Any other deployment, including a plain
                        binary with no Docker, can either install Node and a
                        Playwright harness next to the backend, or hand
                        execution to <code>traceway-runner</code> machines that
                        connect outbound-only and claim work over a long-poll.
                        Scripts always run with an allowlisted environment and
                        never see your server&apos;s secrets.
                      </p>
                    </>
                  ),
                },
                {
                  q: "What stops one slow probe from marking my site down?",
                  a: "The failure threshold. A monitor only transitions to down after the configured number of consecutive failures, and only that transition alerts. Recovery works the same way in reverse: the first successful probe flips it back and sends the all-clear.",
                },
                {
                  q: "What happens if my Traceway instance is down?",
                  a: "The truth is preserved. Runs live in a durable queue, and a probe that outlives its execution window is recorded as missed rather than executed late, so the uptime timeline shows a gap instead of a fabricated 100%. A stale 'is it up' answer is worse than no answer.",
                },
                {
                  q: "Can the status page live on my own domain?",
                  a: (
                    <>
                      <p>
                        Yes. Set the custom domain on the status page, point a
                        CNAME at your Traceway instance, and terminate TLS for
                        that hostname at your reverse proxy. Visitors landing
                        on that host are routed straight to the status page,
                        with no Traceway branding beyond a footer credit.
                      </p>
                    </>
                  ),
                },
                {
                  q: "Is any of this gated behind a paid plan?",
                  a: "Self-hosted Traceway is 100% open source with every monitor type, status pages, and alerting included. On Traceway Cloud, HTTP and TCP monitors and status pages are part of every plan; browser checks are currently a self-hosted feature because they execute your scripts, and your plan sizes how many monitors you can create.",
                },
              ]}
            />
          </div>
        </div>
      </section>
    </main>
  );
}
