import Link from "next/link";
import type { Metadata } from "next";
import { ArrowRight, PhoneCall } from "lucide-react";

import { Chip } from "@/components/chip";
import { SectionHead } from "@/components/section-head";
import { FeatureRow } from "@/components/feature-row";
import { FaqList } from "@/components/faq-list";
import { FinalCTA } from "@/components/final-cta";
import { AuroraBackground } from "@/components/aurora-background";

export const metadata: Metadata = {
  title: "On-Call · Traceway",
  description:
    "PagerDuty-style paging built into your observability stack. Rotating schedules, escalation policies, per-responder notification rules, and a no-login acknowledge link. Your alerts already know something is broken, so let them wake the right person.",
};

export default function OnCallPage() {
  return (
    <main className="relative">
      <section className="hero hero-product relative">
        <AuroraBackground variant="hero" />
        <div className="wrap relative z-10">
          <Chip variant="ok">
            <PhoneCall className="h-3 w-3 inline mr-1" />
            On-Call &amp; Incident Paging
          </Chip>
          <h1 className="mt-6">
            Alerts post to a channel. <em>Pages wake people up.</em>
          </h1>
          <p className="hero-sub">
            Traceway already knows your error rate spiked. On-Call turns that
            into somebody&apos;s phone ringing, then keeps escalating until a
            human acknowledges. Rotating schedules, escalation policies, and
            per-responder notification rules, in the same tool that holds the
            stack trace.
          </p>
          <div className="hero-cta-row">
            <Link
              href="https://docs.tracewayapp.com/learn/on-call"
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
            eyebrow="One incident, not a storm"
            title={
              <>
                A flapping error is <em>one page</em>
              </>
            }
            description="A page is the unit of work, not the message. While it is unresolved, the same condition bumps an event counter instead of opening a new incident, and it never restarts the escalation clock. Sixty firings over half an hour produce one page and exactly the notifications the policy calls for."
            bullets={[
              "Re-fires bump the event count and notify nobody again",
              "The escalation clock is never reset by a duplicate",
              "Open, acknowledged, and resolved, with the level always visible",
              "Resolving releases the key so the next occurrence pages fresh",
            ]}
            image={{
              src: "/images/oncall-pages.png",
              alt: "The Traceway on-call incident queue",
            }}
          />
        </section>

        <section className="wrap">
          <FeatureRow
            reverse
            eyebrow="Schedules"
            title={
              <>
                Rotations that survive <em>real life</em>
              </>
            }
            description="Stack layers to describe how your team actually works. A business hours layer over the whole team, a nights and weekends layer over the smaller group who signed up for it, each restricted to its own window. Later layers take precedence, so the schedule row at the bottom is who actually gets paged."
            bullets={[
              "Daily, weekly, or every N days, with a handoff time you choose",
              "Time-of-day and day-of-week restrictions per layer",
              "Every schedule carries its own timezone, daylight saving included",
              "The timeline renders the stack, so you can see it before it pages",
            ]}
            image={{
              src: "/images/oncall-layer-editor.png",
              alt: "Editing on-call schedule layers and restrictions",
            }}
          />
        </section>

        <section className="wrap">
          <FeatureRow
            eyebrow="Escalation policies"
            title={
              <>
                Escalation stops on <em>acknowledge, not on send</em>
              </>
            }
            description="Delivering a message proves nothing. A policy climbs level by level until a human takes the incident: primary schedule first, secondary after five minutes, the whole team after ten. Targets resolve to people when the step runs, so a schedule step always reaches whoever is on call right now, including an override that started a minute ago."
            bullets={[
              "Target a schedule, a team, a specific person, or a Slack channel",
              "Per-step delay, and a repeat that loops the whole chain",
              "Urgency picks which of the responder's rule chains runs",
              "Each page snapshots its policy, so edits never disturb live incidents",
            ]}
            image={{
              src: "/images/oncall-policy-dialog.png",
              alt: "A three level escalation policy",
            }}
          />
        </section>

        <section className="wrap">
          <FeatureRow
            reverse
            eyebrow="Notification rules"
            title={
              <>
                Nudge first, <em>then get loud</em>
              </>
            }
            description="Each responder owns their own chain per urgency. Slack immediately, a push after two minutes, SMS after five. The whole chain is scheduled the moment the page reaches you, and acknowledging cancels every step that has not fired yet. Take the incident in the first thirty seconds and your phone never rings."
            bullets={[
              "Email, Slack, Pushover, Telegram, and SMS",
              "Separate chains for high and low urgency",
              "Acknowledging cancels the tail of your own chain",
              "No contact methods configured falls back to your account email",
            ]}
            image={{
              src: "/images/oncall-contact-methods.png",
              alt: "Contact methods and per-urgency notification rules",
            }}
          />
        </section>

        <section className="wrap">
          <FeatureRow
            eyebrow="3am ergonomics"
            title={
              <>
                Acknowledge <em>without logging in</em>
              </>
            }
            description="Every delivery to a person carries its own acknowledge link. Tap it from the notification and the escalation stops. No session, no password manager, no SSO round trip at 3am. The link is scoped to acknowledging that one page and nothing else, and it stops working once the page is resolved."
            bullets={[
              "One single-purpose token per delivery, stored hashed",
              "Opening the link is read-only, so email scanners cannot ack for you",
              "Attributed to the person the delivery was addressed to",
              "Acknowledging never requires write access on the project",
            ]}
            image={{
              src: "/images/oncall-ack-link.png",
              alt: "The no-login acknowledge page",
            }}
          />
        </section>

        <section className="wrap">
          <FeatureRow
            reverse
            eyebrow="Wired into your alerts"
            title={
              <>
                The rules you already have, <em>pointed at a human</em>
              </>
            }
            description="Paging is a channel type. Create an escalation channel, point it at a policy, and attach any existing alert rule to it. The rule fires and opens a page instead of sending a message. The Test button opens a real page and runs the real escalation, which is the only honest way to find out whether your rotation and your phone both work."
            bullets={[
              "No separate alerting config to keep in sync",
              "Any rule can page: error rate, latency, metric threshold, missing data",
              "Teams own projects, so an issue shows who is on call for it",
              "Test end to end before you rely on it",
            ]}
            image={{
              src: "/images/oncall-channel-dialog.png",
              alt: "Creating an escalation channel from an alert rule",
            }}
          />
        </section>

        <section className="wrap">
          <FeatureRow
            eyebrow="Who has the pager"
            title={
              <>
                Never ask <em>&ldquo;who is on call?&rdquo;</em> in Slack again
              </>
            }
            description="One screen shows the current responder and the next one up, per team and per schedule. The same answer appears on the issue itself, so when you are staring at a stack trace you already know who owns it and who to pull in."
            bullets={[
              "Current and next on-call, per schedule",
              "Shown on the issue page for the project you are looking at",
              "Teams own projects, so ownership is never ambiguous",
            ]}
            image={{
              src: "/images/oncall-overview.png",
              alt: "Current and next on-call per team and schedule",
            }}
          />
        </section>

        <section className="wrap">
          <FeatureRow
            reverse
            eyebrow="Delivery you can audit"
            title={
              <>
                Every attempt, <em>on the record</em>
              </>
            }
            description="When someone says they never got paged, the delivery log settles it. Each page shows its escalation chain with the current level, and every delivery attempt with its destination and outcome. Nothing sends from inside the request that triggered it, so a crash mid-incident cannot lose a page."
            bullets={[
              "Durable outbox, with the level advance committed alongside it",
              "Retries on a 1, 5, 15, 60 minute backoff before failing terminally",
              "Acknowledging cancels queued deliveries, permanently",
              "Queue depth and oldest pending delivery on the health endpoint",
            ]}
            image={{
              src: "/images/oncall-page-detail.png",
              alt: "Page detail with escalation chain and delivery log",
            }}
          />
        </section>
      </div>

      <FinalCTA
        title={
          <>
            Stop paging <em>the whole team</em>
          </>
        }
        description="Schedules, escalation policies, and per-responder rules, in the same tool that already holds your traces. Included on every plan, and open source if you self-host."
        primary={{
          label: "Read the On-Call docs",
          href: "https://docs.tracewayapp.com/learn/on-call",
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
            title="Questions about on-call and paging"
          />
          <div className="mt-4">
            <FaqList
              items={[
                {
                  q: "Do I still need PagerDuty?",
                  a: (
                    <>
                      <p>
                        Not for the paging loop itself. Traceway covers rotating
                        schedules with stacked layers and restrictions,
                        overrides, multi-level escalation policies with repeat,
                        per-responder notification rules per urgency,
                        deduplicated incidents, and acknowledge links that work
                        without logging in. The difference is that the alert,
                        the stack trace, the trace, and the page all live in one
                        place, so the person you woke up lands on the evidence
                        instead of a link to another tool.
                      </p>
                    </>
                  ),
                },
                {
                  q: "What stops a noisy alert from paging me sixty times?",
                  a: "Deduplication. While a page is unresolved, the same rule and dedup token bump the existing incident rather than opening a new one. The event counter goes up, the escalation clock is untouched, and nobody is notified again. The number of notifications is decided by your escalation policy and your own rule chain, never by how many times the condition fired.",
                },
                {
                  q: "How do I handle a vacation or a swapped shift?",
                  a: "Add an override for the dates. It beats every layer for that window and disappears on its own when the dates pass, so there is no rotation to reshuffle and nothing to remember to put back. Any member of the organization can create one, because covering for a teammate should not need an administrator.",
                },
                {
                  q: "What happens if nobody acknowledges?",
                  a: (
                    <>
                      <p>
                        The policy keeps climbing. Each step waits its
                        configured delay, then notifies the next set of targets.
                        After the last step you can set{" "}
                        <code>Repeat</code> to send the whole chain around again
                        up to five more times. Once the policy is exhausted the
                        page stays open and visible in the queue, but nothing
                        further is sent.
                      </p>
                    </>
                  ),
                },
                {
                  q: "Does SMS work on a self-hosted instance?",
                  a: (
                    <>
                      <p>
                        It needs Twilio credentials on your server:{" "}
                        <code>TWILIO_ACCOUNT_SID</code>,{" "}
                        <code>TWILIO_AUTH_TOKEN</code>, and one sender. Without
                        them SMS is not offered at all, rather than silently
                        accepted and dropped. Slack, Pushover, and Telegram need
                        nothing beyond the webhook or token each responder adds
                        to their own contact method. Email is the one that
                        depends on the server, so configure SMTP before you rely
                        on it: with SMTP off, Traceway logs the message instead
                        of sending it.
                      </p>
                    </>
                  ),
                },
                {
                  q: "Can someone with read-only access acknowledge a page?",
                  a: "Yes, deliberately. Acknowledging and resolving require project read access and nothing more. A responder who gets paged at 3am is never blocked from taking the incident by a permissions check, and the acknowledge link in the notification works without a session at all.",
                },
              ]}
            />
          </div>
        </div>
      </section>
    </main>
  );
}
