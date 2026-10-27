import Link from "next/link";
import type { Metadata } from "next";
import { ArrowRight, Goal } from "lucide-react";

import { Chip } from "@/components/chip";
import { SectionHead } from "@/components/section-head";
import { FeatureRow } from "@/components/feature-row";
import { FaqList } from "@/components/faq-list";
import { FinalCTA } from "@/components/final-cta";
import { AuroraBackground } from "@/components/aurora-background";

export const metadata: Metadata = {
  title: "SLOs · Traceway",
  description:
    "Service level objectives over the telemetry you already send: endpoint availability and latency, any span filter, task success, monitor uptime, a metric held under a limit, or crash-free sessions. Rolling windows, error budgets, burn rates, and multi-window burn-rate alerts that recover on their own.",
};

export default function SlosPage() {
  return (
    <main className="relative">
      <section className="hero hero-product relative">
        <AuroraBackground variant="hero" />
        <div className="wrap relative z-10">
          <Chip variant="ok">
            <Goal className="h-3 w-3 inline mr-1" />
            SLOs, Error Budgets &amp; Burn-Rate Alerts
          </Chip>
          <h1 className="mt-6">
            A threshold says it broke.{" "}
            <em>An objective says how much you can afford.</em>
          </h1>
          <p className="hero-sub">
            An SLO holds an indicator, the share of good requests, spans, task
            runs, probes, metric slices or crash-free sessions, to a target over
            a rolling 7, 30 or 90 day window. What the target leaves over is the
            error budget, and the burn rate says how fast you are spending it.
            Traceway computes all of it from the spans, probes, metrics and
            sessions it already stores, so there is nothing new to instrument.
          </p>
          <div className="hero-cta-row">
            <Link
              href="https://docs.tracewayapp.com/learn/slos"
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

      <div className="band-light">
        <section className="wrap">
          <FeatureRow
            eyebrow="Six kinds of indicator"
            title={
              <>
                Endpoints, spans, tasks, monitors, metrics{" "}
                <em>or crash-free sessions</em>
              </>
            }
            description="Pick what a good event is and Traceway counts it. An endpoint is good when it answers below 500 or faster than a threshold. A task run is good when no exception landed on its span. A monitor probe is good when it came back up. A metric is sliced into one to sixty minute windows and each slice is good when its average, minimum, maximum, sum or last value stays on the right side of a limit, with slices that have no data counting as bad if you want a missing heartbeat to hurt."
            bullets={[
              "Availability or latency for endpoints and spans",
              "Task success from the exception on the run's own span",
              "Monitor uptime straight from the probe results",
              "Metric timeslices: over, under, or missing",
              "Crash-free sessions for browser and mobile apps, per app version",
            ]}
            image={{
              src: "/images/slos-list.png",
              alt: "The Traceway SLOs list with the current SLI, error budget left, burn rate and status of every objective",
            }}
          />
        </section>

        <section className="wrap">
          <FeatureRow
            reverse
            eyebrow="Browser and mobile apps"
            title={
              <>
                Crash-free sessions, <em>per release</em>
              </>
            }
            description="Browser and mobile projects get the objective those platforms live by: the share of sessions that ended without an exception. Pin it to one app version and every rollout gets its own budget, so a release that crashes more than the last one shows up as a breach rather than a hunch. Native crashes uploaded on the next app launch still count, because the worker goes back over the last three days of sessions once a day."
            bullets={[
              "All versions, or one app version per objective",
              "Captured messages count only if you say so",
              "Works for React, Vue, Svelte, React Native, Flutter, Android and iOS",
              "Burn-rate and budget alerts like every other objective",
            ]}
            image={{
              src: "/images/slos-sessions.png",
              alt: "Crash-free session SLOs of a Flutter app in Traceway, one per release, with the error budget left and the burn rate of each",
            }}
          />
        </section>

        <section className="wrap">
          <FeatureRow
            eyebrow="Any span can carry an objective"
            title={
              <>
                Filter spans the way the explorer does,{" "}
                <em>then hold them to a number</em>
              </>
            }
            description="Most SLO products only know services and HTTP routes. A span SLO takes the same filters as the span explorer: service, name, kind, and exact attribute values. A queue consumer, an outbound call to a payment provider, or a GraphQL resolver gets its own objective as long as its spans carry an attribute that identifies it. The Preview button runs the indicator against the last 24 hours before you save, so you see the share of good events and the counts and never guess."
            bullets={[
              "Service, name, kind and up to ten attribute filters",
              "Good when the span status is not error, or faster than a threshold",
              "Preview the last 24 hours before saving",
              "Native SDK runs are stored as spans too, so they count",
            ]}
            image={{
              src: "/images/slos-sheet.png",
              alt: "Creating a span SLO in Traceway with a service, name and attribute filter and a 24-hour preview",
            }}
          />
        </section>

        <section className="wrap">
          <FeatureRow
            reverse
            eyebrow="Error budget and burn rate"
            title={
              <>
                Stop arguing about <em>whether 99.5% is bad</em>
              </>
            }
            description="With a 99.9% target over 30 days and a million requests, a thousand may fail. The detail page shows the current SLI against the target, how much of that budget is left, and the burn rate over the last hour and the last six hours: 1x spends the budget exactly at the end of the window, 14.4x empties a 30-day budget in two days. Charts plot the SLI against the target line and the cumulative share of the budget the range has used. One click opens the requests, spans, task runs or monitor behind the number."
            bullets={[
              "SLI and error budget charts with the target drawn in",
              "Burn rates over 1 hour and 6 hours, colour coded",
              "View bad events: one click to the requests, spans, task runs, probes or sessions behind the number",
              "Rolling 7, 30 or 90 day windows, changed without losing history",
            ]}
            image={{
              src: "/images/slos-detail.png",
              alt: "A Traceway SLO detail page with the SLI chart against its target, the error budget chart, and the burn rate tiles",
            }}
          />
        </section>

        <section className="wrap">
          <FeatureRow
            eyebrow="Alert on the burn, not the blip"
            title={
              <>
                Page on fast burns, chat about slow ones,{" "}
                <em>recover on your own</em>
              </>
            }
            description="Two alert rules read the SLO: Burn Rate fires when the budget burns faster than a threshold over both a long and a short window, so a one-minute spike never pages and a burn that already ended never pages either. Budget Consumed fires when more than a share of the window's budget is gone. Both fire once on the transition, keep their state across restarts, and send a recovery notice when the condition clears. On an escalation channel the recovery resolves the on-call page itself."
            bullets={[
              "Google SRE presets: Fast 14.4x over 1h/5m, Slow 6x over 6h/30m, Long 1x over 3d/6h",
              "Fires on the transition, never re-pages every poll",
              "Recovery notices, and auto-resolved on-call pages",
              "Evaluated from precomputed buckets, no raw scans per poll",
            ]}
            image={{
              src: "/images/slos-rule.png",
              alt: "Configuring an SLO burn-rate alert rule in Traceway with the Fast, Slow and Long presets",
            }}
          />
        </section>

        <section className="wrap">
          <FeatureRow
            reverse
            eyebrow="Computed once, read a thousand times"
            title={
              <>
                Buckets, not scans, <em>on SQLite as well as ClickHouse</em>
              </>
            }
            description="A background worker folds the raw rows into per-minute good/total buckets, refreshes the last ten minutes on every poll so late data still lands in its bucket, and backfills the whole window in chunks after you create or change an objective. The SLI, the budget, the burn rates and the alert rules are sums over those buckets. That is what makes a 90-day objective cheap on an embedded SQLite or DuckDB instance, and it means the history survives the pruning of the raw telemetry behind it."
            bullets={[
              "Per-minute buckets on ClickHouse, SQLite and DuckDB",
              "Late data within ten minutes is absorbed, idempotently",
              "History kept for 120 days whatever the raw retention is",
              "A breached objective still shows where the budget crossed zero",
            ]}
            image={{
              src: "/images/slos-detail-breached.png",
              alt: "A breached Traceway task SLO with an overspent error budget and the budget chart crossing the exhausted line",
            }}
          />
        </section>
      </div>

      <FinalCTA
        title={
          <>
            Objectives in the same tool <em>as the traces that explain them</em>
          </>
        }
        description="SLOs, error budgets, burn-rate alerts, on-call and the spans behind every bad event, in one place. Open source if you self-host."
        primary={{
          label: "Read the SLO docs",
          href: "https://docs.tracewayapp.com/learn/slos",
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
            title="Questions about SLOs"
          />
          <div className="mt-4">
            <FaqList
              items={[
                {
                  q: "How is an SLO different from an alert rule?",
                  a: "A threshold rule looks at the last few minutes and fires when a number crosses a line. An SLO holds a share of good events to a target over weeks, keeps the history, and turns the gap into a budget. The burn-rate rules then alert on how fast that budget is going rather than on the raw rate, which is what keeps a short spike from paging and a slow leak from going unnoticed.",
                },
                {
                  q: "What is a burn rate?",
                  a: "The observed bad rate divided by the rate the target allows. A burn rate of 1 spends the error budget exactly at the end of the window. 14.4 spends a 30-day budget in about two days, which is why the Fast preset pages at 14.4x over the last hour. A window with no traffic burns nothing.",
                },
                {
                  q: "Which indicators can I build an SLO on?",
                  a: "Endpoint availability or latency, any span filter (service, name, kind, exact attribute values) for availability or latency, task success, monitor uptime, and metric timeslices where each slice is good when its aggregate is over or under a limit. Native SDK runs are stored as spans, so they are covered by the span kind. Browser and mobile projects get crash-free sessions instead, since their SDKs report sessions and exceptions rather than spans.",
                },
                {
                  q: "What happens to the history when I change an SLO?",
                  a: "Changing the target or the window keeps every bucket, since both are applied on read. Changing what the indicator measures, or the slice length of a metric SLO, recomputes the history from your retained telemetry; the form warns before you save and the detail page says how much of the window is covered while the backfill runs.",
                },
                {
                  q: "Does it work on the SQLite build?",
                  a: "Yes. The worker computes per-minute buckets on ClickHouse, SQLite and DuckDB alike, and reads sum over buckets rather than scanning raw rows. Buckets are kept for 120 days, so a 90-day objective keeps its history even where the raw telemetry retention is 30 days; what cannot be backfilled is the part of the window older than what is still retained when the SLO is created.",
                },
                {
                  q: "Can an AI agent read my SLOs?",
                  a: "The list, detail, series and breached-count routes are on the agent allowlist, so a read credential, the CLI or the MCP server can inspect them. Dedicated CLI commands and MCP tools are next on the list.",
                },
              ]}
            />
          </div>
        </div>
      </section>
    </main>
  );
}
