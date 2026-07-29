import Link from "next/link";
import type { Metadata } from "next";
import { SquareTerminal, Plug, ArrowRight } from "lucide-react";

import { Chip } from "@/components/chip";
import { FinalCTA } from "@/components/final-cta";
import { AuroraBackground } from "@/components/aurora-background";
import { Eyebrow } from "@/components/eyebrow";
import { Terminal } from "@/components/terminal";
import { CopyCommand } from "@/components/copy-command";
import { GITHUB_URL } from "@/lib/links";

export const metadata: Metadata = {
  title: "CLI · Traceway",
  description:
    "The traceway CLI brings your production exceptions, logs, endpoints, and metrics to the terminal. One binary, gh-style ergonomics for humans, JSON and stable exit codes for agents.",
};

export default function CliPage() {
  return (
    <main className="relative">
      <section className="hero hero-product relative">
        <AuroraBackground variant="hero" />
        <div className="wrap relative z-10">
          <div className="flex flex-col items-center text-center">
            <Chip>
              <SquareTerminal className="h-3 w-3 inline mr-1" />
              CLI
            </Chip>
            <h1 className="mt-6 max-w-[26ch]">
              Production telemetry, <em>in your terminal.</em>
            </h1>
            <p className="hero-sub text-pretty">
              Exceptions, logs, endpoints, and metrics as commands. Tables for
              humans on a TTY, JSON and stable exit codes for scripts and
              agents.
            </p>
            <div className="mt-8 w-full max-w-[640px]">
              <CopyCommand
                size="lg"
                className="w-full"
                command="curl -fsSL https://cli.tracewayapp.com/install.sh | sh"
              />
            </div>
            <p className="dim mt-5 font-mono text-[0.75rem]">
              Or grab a prebuilt binary for macOS, Linux, or Windows with every
              release.
            </p>
            <p className="mt-3 font-mono text-[0.6875rem] uppercase tracking-[0.08em]">
              <Link
                href="https://docs.tracewayapp.com/learn/cli"
                target="_blank"
                rel="noopener noreferrer"
                className="text-fg-2 hover:text-a2 transition-colors"
              >
                Read the CLI docs →
              </Link>
            </p>
          </div>
        </div>
      </section>

      <div className="band-light">
        <section className="wrap">
          <div className="feature-row">
            <div className="feat-copy">
              <Eyebrow>Installation</Eyebrow>
              <h2>
                One binary, <em>anywhere Go runs</em>
              </h2>
              <p>
                One command downloads the latest release for your platform,
                verifies its checksum, and puts <code>traceway</code> on your{" "}
                <code>PATH</code> — no sudo, no package manager. Windows
                archives ship with every release too.
              </p>
              <ul className="feat-bullets">
                <li>One-line installer for macOS and Linux</li>
                <li>Checksum-verified prebuilt binaries, Windows included</li>
                <li>Single static binary, no runtime dependencies</li>
                <li>CLI version tracks the backend release</li>
              </ul>
            </div>
            <Terminal
              title="bash · install"
              lines={[
                {
                  ln: "1",
                  type: "tx",
                  content: (
                    <>
                      <span className="cmd">$</span> curl -fsSL
                      https://cli.tracewayapp.com/install.sh | sh
                    </>
                  ),
                },
                {
                  ln: "2",
                  type: "mute",
                  content: "# detected darwin/arm64 · sha256 verified",
                },
                {
                  ln: "3",
                  type: "tx",
                  content: (
                    <>
                      <span className="cmd">$</span> traceway version
                    </>
                  ),
                },
                {
                  ln: "4",
                  type: "ok",
                  content: "traceway version 1.9.3",
                },
              ]}
            />
          </div>
        </section>

        <section className="wrap">
          <div className="feature-row reverse">
            <div className="feat-copy">
              <Eyebrow>Usage</Eyebrow>
              <h2>
                Ask questions, <em>get answers</em>
              </h2>
              <p>
                Pick a project once, then query it like you&apos;d query{" "}
                <code>gh</code>: grouped exceptions, log search, per-endpoint
                percentiles, metric time series. Output adapts to where it
                goes.
              </p>
              <ul className="feat-bullets">
                <li>Exceptions, logs, endpoints, tasks, traces, metrics</li>
                <li>Tables on a TTY, JSON when piped</li>
                <li>--fields trims responses to what was asked</li>
                <li>Stable error identifiers and exit codes</li>
              </ul>
            </div>
            <Terminal
              title="bash · traceway"
              lines={[
                {
                  ln: "1",
                  type: "tx",
                  content: (
                    <>
                      <span className="cmd">$</span> traceway exceptions list
                      --since 24h
                    </>
                  ),
                },
                {
                  ln: "2",
                  type: "mute",
                  content: "HASH      TYPE       COUNT  LAST SEEN",
                },
                {
                  ln: "3",
                  type: "mute",
                  content: "82b58892  TypeError  412    2m ago",
                },
                {
                  ln: "4",
                  type: "tx",
                  content: (
                    <>
                      <span className="cmd">$</span> traceway logs query --since
                      1h --search &quot;OutOfMemory&quot; | head -1
                    </>
                  ),
                },
                {
                  ln: "5",
                  type: "mute",
                  content:
                    '{"severity":"ERROR","service":"checkout-api","body":"OutOfMemory …"}',
                },
                {
                  ln: "6",
                  type: "ok",
                  content: "# tables for you, JSON for your scripts",
                },
              ]}
            />
          </div>
        </section>

        <section className="wrap">
          <div className="feature-row">
            <div className="feat-copy">
              <Eyebrow>Authentication</Eyebrow>
              <h2>
                Log in once, <em>stay logged in</em>
              </h2>
              <p>
                <code>traceway login</code> opens a browser device flow: approve
                a short code on your instance and the CLI holds a token that
                refreshes itself. No password stored, nothing to paste.
              </p>
              <ul className="feat-bullets">
                <li>Browser device login with auto-refreshing tokens</li>
                <li>Stays signed in for up to 90 days of inactivity</li>
                <li>Personal access tokens for CI and headless boxes</li>
                <li>Profiles for multiple accounts and instances</li>
              </ul>
            </div>
            <Terminal
              title="bash · login"
              lines={[
                {
                  ln: "1",
                  type: "tx",
                  content: (
                    <>
                      <span className="cmd">$</span> traceway login --url
                      https://cloud.tracewayapp.com
                    </>
                  ),
                },
                {
                  ln: "2",
                  type: "mute",
                  content: "# visit https://cloud.tracewayapp.com/device",
                },
                {
                  ln: "3",
                  type: "mute",
                  content: "# and enter the code XKCD-2347",
                },
                {
                  ln: "4",
                  type: "ok",
                  content: "✓ logged in as you@example.com",
                },
                {
                  ln: "5",
                  type: "tx",
                  content: (
                    <>
                      <span className="cmd">$</span> traceway projects use
                      checkout-api
                    </>
                  ),
                },
              ]}
            />
          </div>
        </section>

        <section className="wrap pb-16">
          <Link
            href="/product/mcp"
            className="group mx-auto flex max-w-3xl flex-col items-start gap-4 rounded-2xl p-8 transition-colors sm:flex-row sm:items-center sm:justify-between"
            style={{
              background: "var(--ink-0)",
              border: "1px solid var(--hair-2)",
              boxShadow:
                "0 1px 2px rgba(10,14,24,0.04), 0 12px 30px -18px rgba(10,14,24,0.14)",
            }}
          >
            <div className="flex flex-col gap-2">
              <div className="flex items-center gap-2">
                <Plug className="h-4 w-4 text-a2" />
                <span className="font-mono text-[0.6875rem] uppercase tracking-[0.14em] text-fg-3">
                  Also available
                </span>
              </div>
              <p className="text-lg font-semibold text-fg-0">
                The same binary is an MCP server.
              </p>
              <p className="muted max-w-[48ch] text-pretty">
                Run <code>traceway mcp</code> and Claude Code, Claude Desktop,
                or Cursor get every query as a live tool, reusing your CLI
                login and current project.
              </p>
            </div>
            <span className="inline-flex shrink-0 items-center gap-1.5 font-mono text-[0.8125rem] text-a2 transition-transform group-hover:translate-x-0.5">
              Explore the MCP server
              <ArrowRight className="h-4 w-4" />
            </span>
          </Link>
        </section>
      </div>

      <FinalCTA
        title={
          <>
            Your next debugging session, <em>without leaving the shell.</em>
          </>
        }
        description="Install the CLI, log in, and query your production telemetry."
        primary={{
          label: "Start for free",
          href: "https://cloud.tracewayapp.com/register",
        }}
        secondary={{
          label: "Star on GitHub",
          href: GITHUB_URL,
          external: true,
        }}
      />
    </main>
  );
}
