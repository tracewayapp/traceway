import Link from "next/link";
import Image from "next/image";
import type { Metadata } from "next";
import { Github, BookOpen, Workflow } from "lucide-react";

import { Eyebrow } from "@/components/eyebrow";
import { SectionHead } from "@/components/section-head";
import { StatsStrip } from "@/components/stats-strip";
import { FaqList } from "@/components/faq-list";
import { FinalCTA } from "@/components/final-cta";
import { AuroraBackground } from "@/components/aurora-background";
import { OtelPipelineTabs } from "@/components/otel-pipeline-tabs";
import { SymbolicationBeforeAfter } from "@/components/symbolication-before-after";
import { GITHUB_URL } from "@/lib/links";

export const metadata: Metadata = {
  title: "Stack Trace Symbolication · Traceway",
  description:
    "Open-source, OpenTelemetry-compatible symbolication for JavaScript source maps and Dart/Flutter obfuscation maps. Resolve minified production errors back to the original file, line, and function at ingest. Pure Go, built to be fast.",
};

const BUNDLERS = [
  "Vite",
  "webpack",
  "esbuild",
  "Rollup",
  "Metro",
  "Next.js",
  "SvelteKit",
  "Cloudflare Workers",
];

const LANGUAGE_TILES = [
  {
    src: "/images/frameworks/flutter.png",
    alt: "Flutter",
    w: 250,
    h: 250,
    size: 104,
    z: 30,
    pos: "left-1/2 top-1/2 -translate-x-1/2 -translate-y-1/2",
    engine: true,
  },
  {
    src: "/images/frameworks/javascript.png",
    alt: "JavaScript",
    w: 45,
    h: 45,
    size: 72,
    z: 20,
    pos: "left-[23%] top-[16%]",
  },
  {
    src: "/images/frameworks/node.png",
    alt: "Node",
    w: 52,
    h: 64,
    size: 68,
    z: 20,
    pos: "right-[19%] top-[14%]",
  },
  {
    src: "/images/frameworks/remix.png",
    alt: "Remix",
    w: 45,
    h: 45,
    size: 50,
    z: 10,
    pos: "left-1/2 top-[5%] -translate-x-1/2",
  },
  {
    src: "/images/frameworks/react.png",
    alt: "React",
    w: 45,
    h: 40,
    size: 56,
    z: 10,
    pos: "right-[17%] top-[50%]",
  },
  {
    src: "/images/frameworks/svelte.png",
    alt: "Svelte",
    w: 45,
    h: 45,
    size: 62,
    z: 20,
    pos: "left-[20%] bottom-[16%]",
  },
  {
    src: "/images/frameworks/nextjs.png",
    alt: "Next.js",
    w: 45,
    h: 45,
    size: 60,
    z: 20,
    pos: "right-[37%] bottom-[13%]",
  },
];

export default function SymbolicationPage() {
  return (
    <main className="relative">
      <section className="hero hero-product relative">
        <AuroraBackground variant="hero" />
        <div className="wrap relative z-10">
          <div className="flex flex-col items-center text-center">
            <div
              className="grid place-items-center rounded-2xl"
              style={{
                width: 76,
                height: 76,
                background: "#ffffff",
                border: "1px solid rgba(10, 14, 24, 0.08)",
                boxShadow: "0 12px 28px -16px rgba(0, 0, 0, 0.6)",
              }}
            >
              <Image
                src="/images/frameworks/otel.png"
                alt="OpenTelemetry"
                width={42}
                height={42}
              />
            </div>
            <h1 className="mt-6">
              <span style={{ fontFamily: "var(--font-mono)" }}>
                app.min.js:1:63
              </span>
              <br />
              <em>tells you nothing.</em>
            </h1>
            <p className="hero-sub text-pretty mx-auto">
              Minified bundles reduce every production error to single letters
              on line one. Traceway resolves each frame back to the file, line,
              and function you wrote, the moment the error arrives.
            </p>
            <div className="hero-cta-row justify-center">
              <Link
                href="https://cloud.tracewayapp.com/register"
                className="btn btn-accent"
              >
                Start for free
              </Link>
              <Link
                href={GITHUB_URL}
                className="btn btn-ghost"
                target="_blank"
                rel="noopener noreferrer"
              >
                <Github className="h-4 w-4" />
                View on GitHub
              </Link>
            </div>
            <p className="dim mt-5 font-mono text-[0.75rem]">
              Open source · Pure Go · OpenTelemetry compatible
            </p>
          </div>
        </div>
      </section>

      <div className="band-light">
        <SymbolicationBeforeAfter />

        <section className="wrap py-20">
          <div className="grid items-center gap-12 md:grid-cols-[1fr_0.9fr] md:gap-14">
            <div className="order-2 md:order-1">
              <div
                className="relative hidden h-[360px] md:block"
                aria-hidden="true"
              >
                {LANGUAGE_TILES.map((t) => (
                  <div
                    key={t.alt}
                    title={t.alt}
                    className={`absolute grid place-items-center rounded-2xl border border-hair-2 bg-ink-0 ${t.pos}`}
                    style={{
                      width: t.size,
                      height: t.size,
                      zIndex: t.z,
                      boxShadow: "0 12px 28px -16px rgba(10, 14, 24, 0.25)",
                    }}
                  >
                    <Image
                      src={t.src}
                      alt=""
                      width={t.w}
                      height={t.h}
                      style={{
                        height: Math.round(t.size * (t.engine ? 0.56 : 0.5)),
                        width: "auto",
                      }}
                    />
                  </div>
                ))}
              </div>
              <div className="flex flex-wrap gap-3 md:hidden">
                {LANGUAGE_TILES.filter((t) => !t.engine).map((t) => (
                  <div
                    key={t.alt}
                    title={t.alt}
                    className="grid size-14 shrink-0 place-items-center rounded-2xl border border-hair-2 bg-ink-0"
                    style={{
                      boxShadow: "0 12px 28px -16px rgba(10, 14, 24, 0.25)",
                    }}
                  >
                    <Image
                      src={t.src}
                      alt={t.alt}
                      width={t.w}
                      height={t.h}
                      className="h-8 w-auto"
                    />
                  </div>
                ))}
              </div>
            </div>
            <div className="order-1 md:order-2">
              <Eyebrow>Languages &amp; throughput</Eyebrow>
              <h3
                className="mt-4 text-[22px] leading-tight text-pretty"
                style={{ color: "var(--fg-0)" }}
              >
                JavaScript, Dart, Flutter, and more coming soon.
              </h3>
              <p
                className="mt-4 text-[15px] text-pretty"
                style={{ color: "var(--fg-2)" }}
              >
                One engine reads JavaScript source maps and Dart and Flutter
                obfuscation maps through the same compiled cache, plus every JS
                framework that ships them, from React and Svelte to Next.js and
                Remix. Even on the cheapest box we tested, a 2&nbsp;vCPU Hetzner{" "}
                <code>ccx13</code>, it clears{" "}
                <strong style={{ color: "var(--fg-0)" }}>
                  over 32&times; the stack traces per second
                </strong>{" "}
                of Honeycomb&rsquo;s symbolicator.
                <sup className="ml-0.5" style={{ color: "var(--a2)" }}>
                  *
                </sup>
              </p>
              <p
                className="mt-5 text-[12px]"
                style={{ color: "var(--fg-3)", fontFamily: "var(--font-mono)" }}
              >
                * Measured running as an OpenTelemetry Collector. The full
                benchmark workflow lives in the repo.
              </p>
            </div>
          </div>

          <div className="mt-20">
            <SectionHead
              eyebrow="Performance"
              title={
                <>
                  Symbolication that <em>keeps up with ingest.</em>
                </>
              }
              description="Most symbolicators re-parse source maps on every restart and hold them in RAM until the process dies. Ours compiles each map once, then memory-maps it from disk. The numbers below come straight from the benchmark workflow in the repo, run head to head against Honeycomb's symbolicator on Hetzner hardware."
            />
            <StatsStrip
              stats={[
                { num: "<em>30K+</em>", label: "Stacks/s, hot or cold" },
                {
                  num: "<em>32</em>×",
                  label: "Honeycomb's throughput under churn",
                },
                {
                  num: "<em>18</em>ms",
                  label: "Churn p99, where Honeycomb hits ~3s",
                },
                { num: "<em>361</em>MB", label: "Peak under churn, vs 4.5 GB" },
              ]}
            />
          </div>
        </section>

        <section className="wrap">
          <div className="feature-row">
            <div className="feat-copy">
              <Eyebrow>OpenTelemetry</Eyebrow>
              <h2>
                Bring your own <em>pipeline.</em>
              </h2>
              <p>
                The same engine ships as an OpenTelemetry Collector processor.
                It&rsquo;s a drop-in replacement for Honeycomb&rsquo;s{" "}
                <code>source_map_symbolicator</code>: same component type, same
                attribute contract, same config keys. Existing pipelines and web
                instrumentation work unchanged.
              </p>
              <ul className="feat-bullets">
                <li>Symbolicates spans, span events, and log records</li>
                <li>Cache bounded by disk size, not an entry count in RAM</li>
                <li>Pure Go, so no glibc base image required</li>
                <li>
                  Works in any collector build, with or without Traceway behind
                  it
                </li>
              </ul>
            </div>
            <OtelPipelineTabs />
          </div>
        </section>

        <section className="wrap pb-10">
          <div
            className="rounded-2xl px-6 py-8 md:px-10 md:py-10"
            style={{
              background: "var(--ink-1)",
              border: "1px solid var(--hair)",
            }}
          >
            <div className="grid gap-8 md:grid-cols-[1.2fr_1fr] items-center">
              <div>
                <Eyebrow>Built in the open</Eyebrow>
                <h3 className="mt-3 text-[22px] leading-tight">
                  Read the parser. Run the benchmarks. Self-host all of it.
                </h3>
                <p
                  className="mt-3 text-[14px] max-w-[520px]"
                  style={{ color: "var(--fg-2)" }}
                >
                  The symbolicator, the benchmark harnesses, and the rest of
                  Traceway live in one MIT-licensed repo. Nothing about how your
                  stack traces get resolved is a black box.
                </p>
                <div className="mt-5 flex flex-wrap gap-2">
                  {BUNDLERS.map((b) => (
                    <span key={b} className="tag">
                      {b}
                    </span>
                  ))}
                </div>
              </div>
              <div className="flex flex-wrap gap-x-6 gap-y-3 text-[13px]">
                <Link
                  href={GITHUB_URL}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="inline-flex items-center gap-1.5 hover:text-[color:var(--a2)]"
                  style={{
                    color: "var(--fg-1)",
                    fontFamily: "var(--font-mono)",
                  }}
                >
                  <Github className="h-3.5 w-3.5" />
                  traceway →
                </Link>
                <Link
                  href="https://docs.tracewayapp.com/symbolicator/opentelemetry"
                  target="_blank"
                  rel="noopener noreferrer"
                  className="inline-flex items-center gap-1.5 hover:text-[color:var(--a2)]"
                  style={{
                    color: "var(--fg-1)",
                    fontFamily: "var(--font-mono)",
                  }}
                >
                  <Workflow className="h-3.5 w-3.5" />
                  OTel processor →
                </Link>
                <Link
                  href="https://docs.tracewayapp.com/symbolicator"
                  target="_blank"
                  rel="noopener noreferrer"
                  className="inline-flex items-center gap-1.5 hover:text-[color:var(--a2)]"
                  style={{
                    color: "var(--fg-1)",
                    fontFamily: "var(--font-mono)",
                  }}
                >
                  <BookOpen className="h-3.5 w-3.5" />
                  Documentation →
                </Link>
              </div>
            </div>
          </div>
        </section>
      </div>

      <FinalCTA
        title={
          <>
            Stop debugging <em>app.min.js.</em>
          </>
        }
        description="Upload your source maps once. Every production error after that points at the code you wrote."
        primary={{
          label: "Get Started",
          href: "https://docs.tracewayapp.com/symbolicator/javascript",
        }}
        secondary={{
          label: "Star on GitHub",
          href: GITHUB_URL,
          external: true,
        }}
      />

      <section className="wrap pt-10 pb-24">
        <div className="max-w-3xl mx-auto">
          <SectionHead
            align="center"
            eyebrow="FAQ"
            title="Questions about symbolication"
          />
          <div className="mt-4">
            <FaqList
              items={[
                {
                  q: "What is stack trace symbolication?",
                  a: "Production JavaScript ships minified, so browsers report errors as positions in the bundle, like app.min.js:1:63 inside a function called n. Symbolication uses the source maps from your build to translate those frames back to the original file, line, column, and function name.",
                },
                {
                  q: "How do source maps get into Traceway?",
                  a: "Run npx traceway-sourcemaps --directory ./dist after your build, in CI or locally. It uploads every .map file and its sibling bundle. Under the hood it's a single multipart HTTP endpoint, so plain curl works from any pipeline. Uploads authenticate with a per-project token you can revoke.",
                },
                {
                  q: "Does it work with OpenTelemetry?",
                  a: "Yes, twice over. Errors arriving at Traceway over OTLP are symbolicated at ingest. And the engine ships as a standalone OpenTelemetry Collector processor that is drop-in compatible with Honeycomb's source_map_symbolicator, so you can symbolicate inside your own pipeline, with or without Traceway behind it.",
                },
                {
                  q: "Which bundlers and frameworks are supported?",
                  a: "Anything that emits standard source maps: Vite, webpack, esbuild, Rollup, Metro, and the frameworks built on them. Inline sourceMappingURL data URIs are supported, and maps with missing source file names still resolve line, column, and function name.",
                },
                {
                  q: "Does it symbolicate Dart and Flutter?",
                  a: "Yes. The same engine reads Dart obfuscation maps and resolves obfuscated Flutter and Dart stack traces back to your original symbols. It runs through the same compiled-cache pipeline as JavaScript, and the benchmark above shows both cache modes surviving every load scenario, including the memory-starved OOM box.",
                },
                {
                  q: "Are my source maps exposed publicly?",
                  a: "No. Maps are uploaded to your Traceway instance's blob storage and used server-side only. They are never served to browsers, so you can keep .map files out of your deployed assets entirely.",
                },
                {
                  q: "What happens to errors from a build without maps?",
                  a: "They stay minified but still group correctly. Column numbers keep frames distinct on line one, and resolved function names are excluded from the grouping hash, so existing groups don't reshuffle when you upload maps later.",
                },
              ]}
            />
          </div>
        </div>
      </section>
    </main>
  );
}
