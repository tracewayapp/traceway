import Link from "next/link";
import type { Metadata } from "next";
import { Bot, Bug, Github, Plug, SquareTerminal } from "lucide-react";

import { Chip } from "@/components/chip";
import { SectionHead } from "@/components/section-head";
import { FeatureRow } from "@/components/feature-row";
import { FaqList } from "@/components/faq-list";
import { FinalCTA } from "@/components/final-cta";
import { AuroraBackground } from "@/components/aurora-background";
import { Eyebrow } from "@/components/eyebrow";
import { Terminal } from "@/components/terminal";
import { SkillInstallCommand } from "@/components/skill-install-command";
import { AgentDebugTerminal } from "@/components/agent-debug-terminal";
import { GITHUB_URL } from "@/lib/links";

export const metadata: Metadata = {
  title: "Agent Skills · Traceway",
  description:
    "Install the Traceway agent skills and your coding agent can instrument your app, query production telemetry through the agent-first traceway CLI, and debug issues end to end.",
};

const SKILLS = [
  {
    icon: Plug,
    name: "/traceway-setup",
    description:
      "Instruments a project from scratch. Reads the repo, wires OpenTelemetry exporters for the backend and Traceway SDKs for web and mobile, then verifies that clean, grouped data arrives.",
  },
  {
    icon: Bug,
    name: "/traceway-debug",
    description:
      "Investigates a bug end to end. Pulls grouped exceptions, queries logs around the failure, checks endpoint health and metrics, and correlates it all with your code.",
  },
  {
    icon: SquareTerminal,
    name: "/traceway-install-cli",
    description:
      "Installs the traceway CLI, authenticates against your instance, cloud or self-hosted, and selects the project so every other skill can query it.",
  },
];

const AGENTS = [
  "Claude Code",
  "Cursor",
  "Codex",
  "OpenCode",
  "Gemini CLI",
  "Copilot",
];

export default function AgentSkillsPage() {
  return (
    <main className="relative">
      <section className="hero hero-product relative">
        <AuroraBackground variant="hero" />
        <div className="wrap relative z-10">
          <Chip>
            <Bot className="h-3 w-3 inline mr-1" />
            Agent Skills
          </Chip>
          <h1 className="mt-6">
            AI-first observability. <em>Your agent does the debugging.</em>
          </h1>
          <p className="hero-sub">
            Install the Traceway skills once and your coding agent can
            instrument your app, query production telemetry through the
            agent-first traceway CLI, and take a bug from report to root
            cause.
          </p>
          <div className="hero-cta-row">
            <SkillInstallCommand />
            <Link
              href={GITHUB_URL}
              target="_blank"
              rel="noopener noreferrer"
              className="btn btn-ghost"
            >
              <Github className="h-4 w-4" />
              View on GitHub
            </Link>
          </div>
          <p className="dim mt-5 font-mono text-[0.75rem]">
            Works with Claude Code, Cursor, Codex, and any agent that reads
            SKILL.md.
          </p>
        </div>
      </section>

      <section className="wrap pt-8 pb-24">
        <SectionHead
          eyebrow="The skills"
          title={
            <>
              Three skills, <em>one install.</em>
            </>
          }
          description="Skills are plain Markdown playbooks your agent loads on demand. Each one encodes how a Traceway engineer would do the job, so your agent does it the same way."
        />
        <dl className="grid gap-5 md:grid-cols-3">
          {SKILLS.map((skill) => (
            <div key={skill.name} className="surface-card">
              <dt className="flex items-center gap-3">
                <span
                  className="grid size-8 shrink-0 place-items-center rounded-md border border-hair-2 bg-ink-3 text-a2"
                  aria-hidden
                >
                  <skill.icon className="size-4" />
                </span>
                <span className="font-mono text-sm font-medium text-fg-0">
                  {skill.name}
                </span>
              </dt>
              <dd className="muted mt-4 text-sm leading-relaxed">
                {skill.description}
              </dd>
            </div>
          ))}
        </dl>
      </section>

      <div className="band-light">
        <section className="wrap">
          <div className="feature-row">
            <div className="feat-copy">
              <Eyebrow>Debug</Eyebrow>
              <h2>
                From bug report <em>to root cause</em>
              </h2>
              <p>
                Describe the bug the way a user reported it. The skill walks
                your agent through the same investigation a senior engineer
                would run: exceptions first, then logs around the failure,
                endpoint health, and metrics. It ends in your code, not in a
                dashboard.
              </p>
              <ul className="feat-bullets">
                <li>Grouped exceptions with full stack traces and tags</li>
                <li>Trace-correlated logs reconstruct the failing request</li>
                <li>First-seen timestamps line up regressions with deploys</li>
                <li>The agent reads the failing code and proposes the fix</li>
              </ul>
            </div>
            <AgentDebugTerminal />
          </div>
        </section>

        <section className="wrap">
          <div className="feature-row reverse">
            <div className="feat-copy">
              <Eyebrow>The CLI</Eyebrow>
              <h2>
                A command line <em>designed for agents first</em>
              </h2>
              <p>
                Most CLIs assume a human is typing. The traceway CLI assumes
                an agent is: machine-readable output, stable error envelopes
                with hints, and nothing that blocks a session waiting for
                input. Humans still get tables on a TTY.
              </p>
              <ul className="feat-bullets">
                <li>JSON by default when piped, tables when watched</li>
                <li>--fields trims responses to exactly what was asked</li>
                <li>Stable error identifiers and exit codes to branch on</li>
                <li>Mutations fail fast without --yes, no hung prompts</li>
              </ul>
            </div>
            <Terminal
              title="bash · traceway-cli"
              lines={[
                {
                  ln: "1",
                  type: "tx",
                  content: (
                    <>
                      <span className="cmd">$</span> traceway exceptions list
                      --since 24h | head -1
                    </>
                  ),
                },
                {
                  ln: "2",
                  type: "mute",
                  content:
                    '{"hash":"82b58892","type":"TypeError","count":412}',
                },
                {
                  ln: "3",
                  type: "tx",
                  content: (
                    <>
                      <span className="cmd">$</span> traceway exceptions show
                      82b58892 --fields type,stacktrace
                    </>
                  ),
                },
                {
                  ln: "4",
                  type: "mute",
                  content:
                    '{"type":"TypeError","stacktrace":"src/checkout/session.ts:42 …"}',
                },
                {
                  ln: "5",
                  type: "tx",
                  content: (
                    <>
                      <span className="cmd">$</span> traceway exceptions
                      archive 82b58892
                    </>
                  ),
                },
                {
                  ln: "6",
                  type: "mute",
                  content:
                    '{"error":"confirmation_required","hint":"re-run with --yes","exit_code":2}',
                },
                {
                  ln: "7",
                  type: "ok",
                  content: "# ✓ predictable for agents, readable for humans",
                },
              ]}
            />
          </div>
        </section>

        <section className="wrap">
          <FeatureRow
            eyebrow="Setup"
            title={
              <>
                Set up by the agent, <em>not by the docs</em>
              </>
            }
            description="/traceway-setup reads your repo and picks the right integration path: OpenTelemetry for any backend, Traceway SDKs for browser and mobile. It enforces the rules that keep data clean, then checks that telemetry is actually arriving."
            bullets={[
              "Detects frameworks, services, and background jobs from the repo",
              "OTel for backends, Traceway SDKs for web and mobile",
              "Wires tasks, AI traces, and source maps, not just HTTP",
              "Finishes by verifying grouped endpoints in your dashboard",
            ]}
            image={{
              src: "/images/performance-percentiles-overview.png",
              alt: "Traceway endpoints grouped by route pattern with percentiles",
            }}
          />
        </section>

        <section className="wrap py-16 text-center">
          <div className="max-w-3xl mx-auto flex flex-col items-center gap-5">
            <Eyebrow>Compatibility</Eyebrow>
            <h2>One format, every agent</h2>
            <p style={{ color: "var(--fg-1)", fontSize: 17 }}>
              Skills are plain Markdown in the open SKILL.md format. If your
              agent can read instructions, it can run Traceway. No plugin
              marketplace, no vendor lock-in, and the skills live in the same
              MIT-licensed repo as the rest of Traceway.
            </p>
            <div className="flex flex-wrap items-center justify-center gap-2.5 pt-2">
              {AGENTS.map((agent) => (
                <span key={agent} className="tag">
                  {agent}
                </span>
              ))}
            </div>
          </div>
        </section>
      </div>

      <FinalCTA
        title={
          <>
            Give your agent <em>production context.</em>
          </>
        }
        description="Install the skills, point the CLI at your instance, and let the agent take the next bug."
        primary={{
          label: "Star on GitHub",
          href: GITHUB_URL,
          external: true,
        }}
        secondary={{
          label: "Start for free",
          href: "https://cloud.tracewayapp.com/register",
        }}
      />

      <section className="wrap pt-10 pb-24">
        <div className="max-w-3xl mx-auto">
          <SectionHead
            align="center"
            eyebrow="FAQ"
            title="Questions about agent skills"
          />
          <div className="mt-4">
            <FaqList
              items={[
                {
                  q: "What exactly is an agent skill?",
                  a: "A skill is a Markdown playbook (SKILL.md) that compatible coding agents load when a task matches its description. Traceway ships three: one to instrument a project, one to investigate bugs with production telemetry, and one to install and authenticate the traceway CLI. They are versioned in the open-source repo like any other code.",
                },
                {
                  q: "Which agents are supported?",
                  a: (
                    <>
                      <p>
                        Any agent that understands the SKILL.md convention:
                        Claude Code, Cursor, Codex, OpenCode, Gemini CLI,
                        Copilot, and more.
                      </p>
                      <p>
                        <code>npx skills add tracewayapp/traceway</code>{" "}
                        detects the agents on your machine and installs the
                        skills where each one expects them.
                      </p>
                    </>
                  ),
                },
                {
                  q: "Can an agent damage my production data?",
                  a: "No. The CLI is read-only except for archiving exception groups, and every mutation requires an explicit --yes flag in non-interactive contexts. An agent can query telemetry freely but cannot change it by accident.",
                },
                {
                  q: "Does this work with self-hosted Traceway?",
                  a: "Yes. The CLI authenticates against any instance with traceway login --url, and profiles let one machine talk to several instances, cloud or self-hosted.",
                },
                {
                  q: "Do I need the CLI installed before the skills are useful?",
                  a: "No. The /traceway-install-cli skill handles that: it downloads the right binary for the platform, authenticates, and selects a project. The debug skill calls it automatically when the CLI is missing.",
                },
              ]}
            />
          </div>
        </div>
      </section>
    </main>
  );
}
