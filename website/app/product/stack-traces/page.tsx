import Link from "next/link";
import { ArrowRight, Bug } from "lucide-react";

import { Chip } from "@/components/chip";
import { SectionHead } from "@/components/section-head";
import { FeatureRow } from "@/components/feature-row";
import { FaqList } from "@/components/faq-list";
import { FinalCTA } from "@/components/final-cta";
import { AuroraBackground } from "@/components/aurora-background";

export default function StackTracesPage() {
  return (
    <main className="relative">
      {/* Hero, left aligned */}
      <section className="hero hero-product relative">
        <AuroraBackground variant="hero" />
        <div className="wrap relative z-10">
          <Chip variant="crit">
            <Bug className="h-3 w-3 inline mr-1" />
            Exceptions / Stack Traces
          </Chip>
          <h1 className="mt-6">
            Find and fix issues <em>before your users notice.</em>
          </h1>
          <p className="hero-sub">
            Every exception, grouped by a 10-step normalization pipeline and
            SHA-256 hash. Thousands of duplicates collapse into one issue,
            paired with the session replay or screen recording that caused it.
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

      {/* Every exception grouped and ranked, absorbed from home */}
      <section className="wrap">
        <FeatureRow
          eyebrow="Grouping"
          title={
            <>
              Every exception, <em>grouped and ranked</em>
            </>
          }
          description="Full stack traces, 10-step normalization, SHA-256 grouping. Thousands of duplicates collapse into one ranked issue so you fix what matters first."
          bullets={[
            "Full stack trace capture with file:line",
            "Intelligent error grouping via SHA-256 hash",
            "User impact analysis across sessions",
            "Source map resolution for minified JS",
          ]}
          image={{ src: "/images/exceptions-grouped-ranked.png", alt: "Exception tracking interface" }}
        />
      </section>

      {/* Intelligent grouping */}
      <section className="wrap">
        <FeatureRow
          reverse
          eyebrow="Normalization"
          title={
            <>
              Same bug, <em>same group</em>, every time
            </>
          }
          description="Traceway normalizes stack traces before hashing, so the same logical error gets grouped together even when runtime values differ. Memory addresses, UUIDs, timestamps, numeric IDs, and ANSI codes are stripped before the hash."
          bullets={[
            "Stack trace normalization (10-step pipeline)",
            "Cross-service deduplication",
            "Full context preserved on every occurrence",
          ]}
          image={{ src: "/images/stack-trace.png", alt: "Error grouping interface" }}
        />
      </section>

      {/* Visual context: pair stack traces with session replay */}
      <section className="wrap">
        <FeatureRow
          eyebrow="Visual context"
          title={
            <>
              Pair every stack trace with the <em>replay that caused it</em>
            </>
          }
          description="When a backend exception fires, Traceway attaches the session replay or mobile recording the user was generating at that moment. Open the stack trace and the replay is right there. See what the user did, what the UI looked like, and where the code blew up, in one pane."
          bullets={[
            "Web DOM replay linked by trace ID",
            "Flutter and React Native screen recording",
            "Jump from stack frame → exact frame of the replay",
            "Frontend + backend context in one pane",
          ]}
          image={{ src: "/images/session-replay-viewer.png", alt: "Stack trace paired with session replay" }}
        />
      </section>

      <FinalCTA
        title={
          <>
            Triage <em>faster</em>. Ship <em>safer</em>.
          </>
        }
        description="Connect an SDK, ship an error, see it in Traceway. 5-minute setup."
        primary={{ label: "Get Started", href: "https://docs.tracewayapp.com" }}
      />

      <section className="wrap pt-10 pb-24">
        <div className="max-w-3xl mx-auto">
          <SectionHead align="center" eyebrow="FAQ" title="Questions about stack traces" />
          <div className="mt-4">
            <FaqList
              items={[
                {
                  q: "How does error grouping work?",
                  a: "Traceway applies a 10-step normalization pipeline to every stack trace: extracting the error type, removing absolute file paths, replacing hex addresses, UUIDs, IPs, timestamps, and numeric IDs with placeholders, normalizing whitespace, and stripping ANSI codes. The result is hashed with SHA-256 so identical logical errors always group together, even if runtime values differ.",
                },
                {
                  q: "How does automatic issue ranking work?",
                  a: "Traceway scores each issue based on how often it occurs, how recently it appeared, and how many users are affected. Issues are continuously re-ranked as new data comes in, so regressions and trending problems surface immediately, with no manual triage required.",
                },
                {
                  q: "How does error grouping handle different environments?",
                  a: "Traceway normalizes stack traces by removing runtime-specific values like memory addresses, file paths, UUIDs, and timestamps before hashing. This means the same bug produces the same group regardless of which server or environment it occurred on.",
                },
                {
                  q: "Can I track frontend and mobile errors alongside the backend?",
                  a: "Yes. Web (Next.js, Svelte, Remix) and mobile (Flutter, React Native) exceptions land in the same dashboard as your backend ones, and each one carries the session replay or screen recording the user was generating when it fired. Open the stack trace and the exact frame of the replay is one click away. Source maps resolve minified web traces back to original source.",
                },
              ]}
            />
          </div>
        </div>
      </section>
    </main>
  );
}
