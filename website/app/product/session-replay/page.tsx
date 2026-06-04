import Link from "next/link";
import { ArrowRight, Video, Zap, ShieldCheck } from "lucide-react";

import { Chip } from "@/components/chip";
import { SectionHead } from "@/components/section-head";
import { FeatureRow } from "@/components/feature-row";
import { BentoGrid, BentoCell } from "@/components/bento-grid";
import { FaqList } from "@/components/faq-list";
import { FinalCTA } from "@/components/final-cta";
import { AuroraBackground } from "@/components/aurora-background";

export default function SessionReplayPage() {
  return (
    <main className="relative">
      <section className="hero hero-product relative">
        <AuroraBackground variant="hero" />
        <div className="wrap relative z-10">
          <Chip>
            <Video className="h-3 w-3 inline mr-1" />
            Session Replay
          </Chip>
          <h1 className="mt-6">
            See exactly <em>what the user did.</em>
          </h1>
          <p className="hero-sub">
            Traceway captures ~10 seconds of user activity before every error.
            Clicks, scrolls, and form interactions are attached to exceptions
            automatically. No manual reproduction needed.
          </p>
          <div className="hero-cta-row">
            <Link href="https://docs.tracewayapp.com" className="btn btn-accent">
              Get Started <ArrowRight className="h-4 w-4" />
            </Link>
            <Link href="https://cloud.tracewayapp.com/register" className="btn btn-ghost">
              Try Traceway Cloud
            </Link>
          </div>
        </div>
      </section>

      {/* Absorbed from home */}
      <section className="wrap">
        <FeatureRow
          eyebrow="Pre-error capture"
          title={
            <>
              See exactly <em>what the user did</em>
            </>
          }
          description="Traceway captures ~10 seconds of user activity before every error. Clicks, scrolls, and form interactions are attached to exceptions automatically, with no manual reproduction needed."
          bullets={[
            "Pre-error activity capture",
            "Automatic attachment to exceptions",
            "Clicks, scrolls, and form interactions",
            "Page navigation timeline",
          ]}
          image={{ src: "/images/session-replay-viewer.png", alt: "Session replay interface" }}
        />
      </section>

      {/* Watch moments before error */}
      <section className="wrap">
        <FeatureRow
          reverse
          eyebrow="DOM replay"
          title="Watch the moments before every error"
          description="Traceway records DOM changes in the browser, not screenshots. You can scrub through the timeline, pause on the exact moment things went wrong, and see the user's real interaction."
          bullets={[
            "Lightweight DOM snapshot recording",
            "Scrub through exact user timeline",
            "Console and network overlays",
            "Trace ID links every replay to the backend",
          ]}
          image={{ src: "/images/session-replay-scrubbing-timeline.png", alt: "DOM replay scrubbing" }}
        />
      </section>

      {/* 2-card bento */}
      <section className="wrap py-10">
        <SectionHead
          eyebrow="Automatic & private"
          title={
            <>
              Zero-config capture. <em>Privacy-first by default.</em>
            </>
          }
        />
        <BentoGrid>
          <BentoCell
            size="wide"
            icon={Zap}
            title="Attached to every exception, automatically"
            iconColor="var(--a4)"
          >
            <p>
              No manual setup or reproduction steps. Every exception gets a
              replay attached automatically. Click into any issue and watch
              exactly what the user did in the ~10 seconds before things
              broke.
            </p>
          </BentoCell>
          <BentoCell
            size="tall"
            icon={ShieldCheck}
            title="Built for privacy"
            iconColor="var(--ok)"
          >
            <p>
              Passwords and any element marked{" "}
              <code>data-sensitive</code> are masked by default. Need stricter
              control? Drop <code>class=&quot;rr-mask&quot;</code> on any
              element to blank its text, or <code>rr-block</code> to hide it
              entirely. No SDK configuration needed.
            </p>
          </BentoCell>
        </BentoGrid>
      </section>

      <FinalCTA
        title={
          <>
            Stop guessing. <em>Watch it happen.</em>
          </>
        }
        description="Session replay is included on every plan, on Cloud and self-hosted."
        primary={{ label: "Get Started", href: "https://docs.tracewayapp.com" }}
      />

      <section className="wrap pt-10 pb-24">
        <div className="max-w-3xl mx-auto">
          <SectionHead align="center" eyebrow="FAQ" title="Questions about session replay" />
          <div className="mt-4">
            <FaqList
              items={[
                {
                  q: "What is session replay?",
                  a: "Session replay records DOM changes in the browser. When an error occurs, Traceway captures approximately 10 seconds of user activity leading up to the exception: clicks, scrolls, form interactions, and page navigations. The replay is attached to the exception automatically, so you can see exactly what the user did without asking them to reproduce the issue.",
                },
                {
                  q: "How does session replay work?",
                  a: "Traceway records DOM changes in the browser (not screenshots). When an exception occurs, approximately 10 seconds of user activity is captured and attached to the error automatically. The trace ID is propagated to the backend, so the replay links directly to the server-side error.",
                },
                {
                  q: "Does session replay affect performance?",
                  a: "The recording adds minimal overhead. It runs a circular DOM buffer in the background and only the last ~10 seconds are retained. Data is sent only when an exception triggers capture, so there's no continuous upload.",
                },
                {
                  q: "What about user privacy?",
                  a: (
                    <>
                      <p>
                        Sensitive inputs are masked by default. Password fields, any{" "}
                        <code>data-sensitive</code> element, and anything with{" "}
                        <code>aria-masked</code> is never captured.
                      </p>
                      <p>
                        For element-level control, Traceway honors rrweb&apos;s standard class
                        hooks:{" "}
                        <code>class=&quot;rr-mask&quot;</code> blocks all text inside the
                        element (including children),{" "}
                        <code>class=&quot;rr-block&quot;</code> hides the element entirely
                        behind a neutral placeholder, and{" "}
                        <code>class=&quot;rr-ignore&quot;</code> records input interactions
                        without capturing the typed characters. No SDK configuration needed,
                        just drop the class on the element.
                      </p>
                    </>
                  ),
                },
              ]}
            />
          </div>
        </div>
      </section>
    </main>
  );
}
