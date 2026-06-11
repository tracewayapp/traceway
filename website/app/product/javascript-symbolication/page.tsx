import Link from "next/link";
import Image from "next/image";
import type { Metadata } from "next";
import { ArrowRight, Github, BookOpen, Workflow } from "lucide-react";

import { Eyebrow } from "@/components/eyebrow";
import { SectionHead } from "@/components/section-head";
import { StatsStrip } from "@/components/stats-strip";
import { FeatureRow } from "@/components/feature-row";
import { FaqList } from "@/components/faq-list";
import { FinalCTA } from "@/components/final-cta";
import { AuroraBackground } from "@/components/aurora-background";
import { OtelPipelineTabs } from "@/components/otel-pipeline-tabs";
import { SymbolicationBeforeAfter } from "@/components/symbolication-before-after";
import { GITHUB_URL } from "@/lib/links";

export const metadata: Metadata = {
  title: "JavaScript Stack Trace Symbolication · Traceway",
  description:
    "Open-source, OpenTelemetry-compatible source map symbolication. Resolve minified production errors back to the original file, line, and function at ingest. Pure Go, built to be fast.",
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

export default function JavaScriptSymbolicationPage() {
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
                href="https://docs.tracewayapp.com/client/js-sdk/sourcemap-upload"
                className="btn btn-accent"
              >
                Upload your source maps <ArrowRight className="h-4 w-4" />
              </Link>
              <Link href={GITHUB_URL} className="btn btn-ghost" target="_blank" rel="noopener noreferrer">
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
          <SectionHead
            eyebrow="Performance"
            title={
              <>
                Symbolication that <em>keeps up with ingest.</em>
              </>
            }
            description="Most symbolicators re-parse source maps on every restart and hold them in RAM until the process dies. Ours compiles each map once, then memory-maps it from disk. Numbers below come from the benchmark workflows in the repo."
          />
          <StatsStrip
            stats={[
              { num: "<em>&lt;1</em>µs", label: "To open a compiled source map" },
              { num: "<em>&lt;1</em>ms", label: "p99 lookup with a cold cache" },
              { num: "<em>3</em>×", label: "Faster bundle parsing than SWC" },
              { num: "<em>0</em>", label: "Maps re-parsed after a restart" },
            ]}
          />
          <div className="mt-14 grid gap-12 md:grid-cols-2">
            <div className="border-t border-hair pt-8">
              <p className="font-mono text-[0.6875rem] uppercase tracking-[0.14em] text-fg-3">
                Why it stays fast
              </p>
              <ul className="mt-4 space-y-3 text-[14px]" style={{ color: "var(--fg-2)" }}>
                <li>
                  <strong style={{ color: "var(--fg-1)" }}>Parse once, compile to .tw.</strong>{" "}
                  Each map and bundle compiles into a binary format on first
                  use. Every lookup after that opens the file in under a
                  microsecond and binary-searches it.
                </li>
                <li>
                  <strong style={{ color: "var(--fg-1)" }}>Disk is the budget, not RAM.</strong>{" "}
                  Compiled maps are memory-mapped, so resident memory tracks
                  the hot set. A corpus of thousands of bundles costs disk
                  space, not heap.
                </li>
                <li>
                  <strong style={{ color: "var(--fg-1)" }}>Cold lookups don&rsquo;t hurt.</strong>{" "}
                  An in-RAM LRU churns the garbage collector every time an old
                  release throws. The mmap cache holds p99 under a millisecond
                  on the long tail.
                </li>
                <li>
                  <strong style={{ color: "var(--fg-1)" }}>Restarts warm from disk.</strong>{" "}
                  Nothing is re-parsed when the process comes back. The cache
                  is already there.
                </li>
              </ul>
            </div>
            <div className="border-t border-hair pt-8">
              <p className="font-mono text-[0.6875rem] uppercase tracking-[0.14em] text-fg-3">
                The fine print
              </p>
              <ul className="mt-4 space-y-3 text-[14px]" style={{ color: "var(--fg-3)" }}>
                <li>
                  <strong style={{ color: "var(--fg-1)" }}>Two parser engines.</strong>{" "}
                  The default build is pure Go and parses bundles with{" "}
                  <code>dop251/goja</code>. A build flag swaps in{" "}
                  <code>oxc</code>, which{" "}
                  <Link
                    href="https://oxc.rs/docs/guide/usage/parser.html"
                    target="_blank"
                    rel="noopener noreferrer"
                    className="underline decoration-dotted underline-offset-4 hover:text-[color:var(--a2)]"
                  >
                    parses 3x faster than SWC
                  </Link>
                  .
                </li>
                <li>
                  <strong style={{ color: "var(--fg-1)" }}>Two cache modes.</strong>{" "}
                  Hold parsed maps in memory, or compile them to{" "}
                  <code>.tw</code> files and memory-map them from disk.
                </li>
                <li>
                  <strong style={{ color: "var(--fg-1)" }}>Not memory bound.</strong>{" "}
                  With oxc and the disk cache, the corpus is a disk budget: the
                  cache holds as many maps as the disk fits. Competing
                  symbolicators cache parsed maps in RAM, so their corpus caps
                  out at what the heap can hold.
                </li>
                <li>
                  <strong style={{ color: "var(--fg-1)" }}>Benchmarked in the open.</strong>{" "}
                  Parser numbers come from <code>go test -bench</code> over
                  webpack, Metro, and Preact fixtures plus 1&nbsp;MB and
                  5&nbsp;MB synthetic bundles. Cache numbers come from a sweep
                  of 1,000 to 12,000 bundles on a 2&nbsp;vCPU box. Both
                  workflows are in the repo; run them on your fork.
                </li>
                <li>
                  <strong style={{ color: "var(--fg-1)" }}>MIT licensed, actually open source.</strong>{" "}
                  The parsers, the cache, the collector processor, and the
                  whole backend live in one public repo. No open core, no
                  enterprise tier where symbolication really lives.
                </li>
              </ul>
            </div>
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
                attribute contract, same config keys. Existing pipelines and
                web instrumentation work unchanged.
              </p>
              <ul className="feat-bullets">
                <li>Symbolicates spans, span events, and log records</li>
                <li>Cache bounded by disk size, not an entry count in RAM</li>
                <li>Pure Go, so no glibc base image required</li>
                <li>Works in any collector build, with or without Traceway behind it</li>
              </ul>
            </div>
            <OtelPipelineTabs />
          </div>
        </section>

        <section className="wrap pb-10">
          <div
            className="rounded-2xl px-6 py-8 md:px-10 md:py-10"
            style={{ background: "var(--ink-1)", border: "1px solid var(--hair)" }}
          >
            <div className="grid gap-8 md:grid-cols-[1.2fr_1fr] items-center">
              <div>
                <Eyebrow>Built in the open</Eyebrow>
                <h3 className="mt-3 text-[22px] leading-tight">
                  Read the parser. Run the benchmarks. Self-host all of it.
                </h3>
                <p className="mt-3 text-[14px] max-w-[520px]" style={{ color: "var(--fg-2)" }}>
                  The symbolicator, the benchmark harnesses, and the rest of
                  Traceway live in one MIT-licensed repo. Nothing about how
                  your stack traces get resolved is a black box.
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
                  style={{ color: "var(--fg-1)", fontFamily: "var(--font-mono)" }}
                >
                  <Github className="h-3.5 w-3.5" />
                  traceway →
                </Link>
                <Link
                  href="https://github.com/tracewayapp/traceway/tree/main/backend/app/symbolicator/otelprocessor"
                  target="_blank"
                  rel="noopener noreferrer"
                  className="inline-flex items-center gap-1.5 hover:text-[color:var(--a2)]"
                  style={{ color: "var(--fg-1)", fontFamily: "var(--font-mono)" }}
                >
                  <Workflow className="h-3.5 w-3.5" />
                  OTel processor →
                </Link>
                <Link
                  href="https://docs.tracewayapp.com/client/js-sdk/sourcemap-upload"
                  target="_blank"
                  rel="noopener noreferrer"
                  className="inline-flex items-center gap-1.5 hover:text-[color:var(--a2)]"
                  style={{ color: "var(--fg-1)", fontFamily: "var(--font-mono)" }}
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
          href: "https://docs.tracewayapp.com/client/js-sdk/sourcemap-upload",
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
