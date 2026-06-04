import Link from "next/link";
import {
  Smartphone,
  Github,
  ShieldCheck,
  Terminal as TerminalIcon,
  MousePointer2,
  BookOpen,
  Globe,
  Route,
  Bookmark,
} from "lucide-react";

import { Chip } from "@/components/chip";
import { Eyebrow } from "@/components/eyebrow";
import { SectionHead } from "@/components/section-head";
import { StatsStrip } from "@/components/stats-strip";
import { FinalCTA } from "@/components/final-cta";
import { Terminal } from "@/components/terminal";
import { AuroraBackground } from "@/components/aurora-background";
import { FeatureRow } from "@/components/feature-row";
import { BentoGrid, BentoCell } from "@/components/bento-grid";
import { HeroEmailCTA } from "@/components/hero-email-cta";
import { FlutterReplayShowcase } from "@/components/flutter-replay-showcase";

export default function FlutterSessionReplayPage() {
  return (
    <main className="relative">
      {/* 1. HERO: centered, matches home page layout */}
      <section className="hero hero-product relative">
        <AuroraBackground variant="hero" />
        <div className="wrap relative z-10">
          <div className="text-center max-w-3xl mx-auto flex flex-col items-center">
            <Chip>
              <Smartphone className="h-3 w-3 inline mr-1" />
              Flutter Session Replay
            </Chip>
            <h1 className="mt-6">
              See the crash.
              <br />
              <em>Not just the Stack Trace.</em>
            </h1>
            <p className="hero-sub">
              Full-screen session recording, pinned to every stack trace. Four
              lines of setup. Zero frame drops on your app.
            </p>
            <div className="mt-10 w-full">
              <HeroEmailCTA />
            </div>
          </div>
        </div>
      </section>

      {/* 1a. Showcase: stack trace paired with phone replay */}
      <FlutterReplayShowcase />

      {/* 1b. Free-tier positioning section */}
      <section className="wrap py-16">
        <div className="max-w-3xl">
          <SectionHead
            eyebrow="Free tier"
            title={
              <>
                Same crash reporting. <em>Full replay. Free forever.</em>
              </>
            }
            description={
              <>
                Same error grouping, same impact ranking, same alerts you&rsquo;d
                expect from{" "}
                <span style={{ fontWeight: 900, color: "#7f5cfc" }}>Sentry</span>,{" "}
                with the full replay included on the free tier. No per-replay
                billing.
              </>
            }
          />
          <div className="mt-8 flex flex-wrap items-center gap-3">
            <Link
              href="/cloud"
              className="inline-flex"
              aria-label="See pricing"
            >
              <Chip variant="ok">
                <span
                  style={{
                    fontFamily: "var(--font-mono)",
                    letterSpacing: "0.04em",
                  }}
                >
                  10,000 replays / month, free forever
                </span>
              </Chip>
            </Link>
            <span
              className="text-[12px]"
              style={{
                color: "var(--fg-3)",
                fontFamily: "var(--font-mono)",
              }}
            >
              1 replay = 1 crash clip, up to 10sec, retained 30 days.{" "}
              <Link
                href="/cloud"
                className="underline decoration-dotted underline-offset-4 hover:text-[color:var(--a2)]"
              >
                See pricing →
              </Link>
            </span>
          </div>
        </div>
      </section>

      {/* 2. SEE IT: product screenshot, the single biggest missing piece */}
      <section className="wrap pt-10">
        <FeatureRow
          eyebrow="The actual player"
          title={
            <>
              Press play on <em>the moment it broke.</em>
            </>
          }
          description="Every exception in the Traceway dashboard carries its replay. Open the stack trace, press play, and watch the last seconds of your user's session leading up to the crash, synced to the timeline."
          bullets={[
            "Scrub through the full session timeline",
            "Replay ID linked from every stack trace",
            "Tap, route, network, and console overlays",
            "Share a single URL with your team",
          ]}
          image={{
            src: "/images/session-replay-viewer.png",
            alt: "Traceway session replay viewer with stack trace",
          }}
        />
      </section>

      {/* 3. THE 4 LINES: masking on by default */}
      <section className="wrap py-20">
        <div className="grid gap-12 md:grid-cols-[1fr_1.1fr] items-center">
          <div>
            <Eyebrow>Setup</Eyebrow>
            <h2 className="mt-3">
              Paste four lines. <em>Ship.</em>
            </h2>
            <p className="muted mt-4 max-w-[460px]">
              Recording starts on the first frame. Every exception carries the
              replay ID automatically. Sensitive widgets are masked by default.
              Wrap anything extra in <code>TracewayMask</code>.
            </p>
            <p
              className="mt-4 text-[13px]"
              style={{
                color: "var(--fg-3)",
                fontFamily: "var(--font-mono)",
              }}
            >
              // <code>flutter pub add traceway</code> to install.
            </p>
          </div>
          <Terminal
            title="main.dart"
            lines={[
              { type: "cmd", content: "Traceway.run(" },
              {
                type: "tx",
                content:
                  "  connectionString: 'token@cloud.tracewayapp.com/api/report',",
              },
              {
                type: "tx",
                content:
                  "  options: TracewayOptions(replay: ReplayOptions.maskAll),",
              },
              { type: "cmd", content: "  child: MyApp());" },
            ]}
            showCursor
          />
        </div>
      </section>

      {/* 4. WHAT'S IN A REPLAY */}
      <section className="wrap py-10">
        <SectionHead
          eyebrow="What you actually see"
          title={
            <>
              A video is <em>the start.</em> The context is the rest.
            </>
          }
          description="Every replay carries the surrounding signal, so you don't just watch what happened, you see why."
        />
        <BentoGrid>
          <BentoCell
            size="med"
            icon={MousePointer2}
            title="Taps & gestures"
            iconColor="var(--a2)"
          >
            <p>
              Every tap and gesture rendered onto the recording, frame-synced
              with the last 10 seconds before the crash.
            </p>
          </BentoCell>
          <BentoCell
            size="med"
            icon={TerminalIcon}
            title="Console logs"
            iconColor="var(--warn)"
          >
            <p>
              Every <code>print</code> and <code>debugPrint</code> from the
              last 10 seconds, captured via a Zone hook with no manual wiring.
              Capped at 200 lines.
            </p>
          </BentoCell>
          <BentoCell
            size="med"
            icon={Globe}
            title="HTTP requests"
            iconColor="var(--a1)"
          >
            <p>
              Method, URL, status, duration, and byte counts for every dart:io
              HTTP call. Catches <code>package:http</code>, Dio, Firebase, and
              anything on the platform client, with no per-call instrumentation.
            </p>
          </BentoCell>
          <BentoCell
            size="med"
            icon={Route}
            title="Navigation timeline"
            iconColor="var(--a2)"
          >
            <p>
              Every push, pop, and replace from any <code>Navigator</code>.
              Attach <code>Traceway.navigatorObserver</code> once and the
              route history rides with every crash.
            </p>
          </BentoCell>
          <BentoCell
            size="med"
            icon={Bookmark}
            title="Custom actions"
            iconColor="var(--warn)"
          >
            <p>
              Tag your own breadcrumbs like <code>cart.add_item</code> and{" "}
              <code>auth.login_succeeded</code> with{" "}
              <code>Traceway.recordAction</code>. Capped at 200 entries.
            </p>
          </BentoCell>
          <BentoCell
            size="med"
            icon={TerminalIcon}
            title="Full stack trace"
            iconColor="var(--a1)"
          >
            <p>
              Caught <code>FlutterError</code>s and uncaught async exceptions,
              symbolicated and stitched to the same timeline as the replay.
            </p>
          </BentoCell>
        </BentoGrid>
      </section>

      {/* 6. BENCHMARKS */}
      <section className="wrap py-20">
        <SectionHead
          eyebrow="Measured on real hardware"
          title={
            <>
              No frame drops. <em>No battery spike.</em>
            </>
          }
          description="Benchmarked on Pixel 5/6/8 and iPhone 8/14 Pro/16 Pro via Firebase Test Lab: 10 workloads × 4 SDK configs per device. Full harness open source; re-run it on your own tier. Tail latencies included because p50 is where bugs hide."
        />
        <StatsStrip
          stats={[
            { num: "<em>+0</em>MB", label: "Memory overhead when idle (loaded, not recording)" },
            { num: "<em>+16</em>MB", label: "Median RAM during active screen capture" },
            { num: "<em>&lt;1</em>ms", label: "Exception capture latency on iOS" },
            { num: "<em>0%</em>", label: "Steady-state wall-clock impact" },
          ]}
        />
        <div className="mt-12 grid gap-6 md:grid-cols-2 max-w-4xl mx-auto">
          <div
            className="rounded-xl px-6 py-6"
            style={{
              background: "var(--ink-3)",
              border: "1px solid var(--hair-2)",
            }}
          >
            <Eyebrow>What we guarantee</Eyebrow>
            <ul
              className="mt-4 space-y-3 text-[14px]"
              style={{ color: "var(--fg-2)" }}
            >
              <li>
                <strong style={{ color: "var(--fg-1)" }}>Idle is free.</strong>{" "}
                With <code>screenCapture: false</code> or before the first
                recording, RSS overhead stays under 10&nbsp;MB on every tested
                device, and is typically zero.
              </li>
              <li>
                <strong style={{ color: "var(--fg-1)" }}>Active recording stays small.</strong>{" "}
                Median memory overhead during capture is under 20&nbsp;MB; the
                worst measured case across all devices held below 80&nbsp;MB
                even under bursty video + exception workloads.
              </li>
              <li>
                <strong style={{ color: "var(--fg-1)" }}>Exception capture beats a frame.</strong>{" "}
                Sub-millisecond on iOS, under 15&nbsp;ms on Android. Both fit
                inside a single 60 Hz frame (16.7&nbsp;ms), so capturing an
                exception cannot drop a frame in steady state.
              </li>
              <li>
                <strong style={{ color: "var(--fg-1)" }}>Disk persistence is free.</strong>{" "}
                Writing recordings to disk consumes the same RAM as in-memory
                only, with no extra cost for offline-safe shipping.
              </li>
            </ul>
          </div>
          <div
            className="rounded-xl px-6 py-6"
            style={{
              background: "var(--ink-3)",
              border: "1px solid var(--hair-2)",
            }}
          >
            <Eyebrow>The fine print</Eyebrow>
            <ul
              className="mt-4 space-y-3 text-[14px]"
              style={{ color: "var(--fg-3)" }}
            >
              <li>
                Numbers come from a single GitHub Actions run on Firebase Test
                Lab hardware. Methodology and raw data live in the SDK repo.
              </li>
              <li>
                Frame-timing carries enough variance per scenario that we don&rsquo;t
                claim improvements; we claim &quot;no measurable regression on any
                tested workload&quot;.
              </li>
              <li>
                Worst-case memory peaks come from synthetic burst tests (5
                exceptions per second during video playback). Apps that throw
                1–2 exceptions per session sit at the median.
              </li>
              <li>
                Run the benchmark yourself with{" "}
                <Link
                  href="https://github.com/tracewayapp/traceway-flutter/blob/main/.github/workflows/benchmark.yml"
                  className="underline decoration-dotted underline-offset-4 hover:text-[color:var(--a2)]"
                >
                  Performance Benchmarks
                </Link>{" "}
                on your fork.
              </li>
            </ul>
          </div>
        </div>
      </section>

      {/* 7. TRUST ROW */}
      <section className="wrap pb-10">
        <div
          className="rounded-2xl px-6 py-8 md:px-10 md:py-10"
          style={{
            background: "linear-gradient(180deg, var(--ink-3), var(--ink-2))",
            border: "1px solid var(--hair-2)",
          }}
        >
          <div className="grid gap-8 md:grid-cols-[1.2fr_1fr] items-center">
            <div>
              <Eyebrow>Built in the open</Eyebrow>
              <h3
                className="mt-3 text-[22px] leading-tight"
                style={{
                  fontFamily: "var(--font-display)",
                  letterSpacing: "-0.01em",
                }}
              >
                Read the SDK. Run the benchmarks.{" "}
                <em style={{ color: "var(--a2)" }}>Self-host the server.</em>
              </h3>
              <p
                className="mt-3 text-[14px] max-w-[520px]"
                style={{ color: "var(--fg-2)" }}
              >
                Every part of Traceway is open source: the Flutter SDK, the Go
                backend, the dashboard. Nothing about your crash data is a
                black box.
              </p>
            </div>
            <div className="flex flex-wrap gap-x-6 gap-y-3 text-[13px]">
              <Link
                href="https://github.com/tracewayapp/traceway-flutter"
                target="_blank"
                rel="noopener noreferrer"
                className="inline-flex items-center gap-1.5 hover:text-[color:var(--a2)]"
                style={{ color: "var(--fg-1)", fontFamily: "var(--font-mono)" }}
              >
                <Github className="h-3.5 w-3.5" />
                traceway-flutter →
              </Link>
              <Link
                href="https://github.com/tracewayapp/traceway"
                target="_blank"
                rel="noopener noreferrer"
                className="inline-flex items-center gap-1.5 hover:text-[color:var(--a2)]"
                style={{ color: "var(--fg-1)", fontFamily: "var(--font-mono)" }}
              >
                <Github className="h-3.5 w-3.5" />
                traceway (server) →
              </Link>
              <Link
                href="https://docs.tracewayapp.com/client/flutter"
                target="_blank"
                rel="noopener noreferrer"
                className="inline-flex items-center gap-1.5 hover:text-[color:var(--a2)]"
                style={{ color: "var(--fg-1)", fontFamily: "var(--font-mono)" }}
              >
                <BookOpen className="h-3.5 w-3.5" />
                Documentation →
              </Link>
              <Link
                href="/privacy-policy"
                className="inline-flex items-center gap-1.5 hover:text-[color:var(--a2)]"
                style={{ color: "var(--fg-1)", fontFamily: "var(--font-mono)" }}
              >
                <ShieldCheck className="h-3.5 w-3.5" />
                Privacy & DPA →
              </Link>
            </div>
          </div>
        </div>
      </section>

      {/* 8. FINAL CTA */}
      <FinalCTA
        title={
          <>
            Ship it <em>before your next release.</em>
          </>
        }
        description="10,000 replays every month. Free. Forever. No card required. One crash clip = one replay, retained 30 days."
        primary={{
          label: "Create your project",
          href: "https://cloud.tracewayapp.com/register?framework=flutter",
        }}
        secondary={{
          label: "Read the docs",
          href: "https://docs.tracewayapp.com/client/flutter",
          external: true,
        }}
      />
    </main>
  );
}
