import Link from "next/link";
import type { Metadata } from "next";
import { Bot, Plug, SquareTerminal } from "lucide-react";

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
      "Reads your repo and wires it up: OpenTelemetry for the backend, Traceway SDKs for web and mobile. Verifies data arrives clean and grouped.",
    tags: ["OTel backends", "Web + mobile SDKs", "Source maps", "Verification"],
  },
  {
    icon: SquareTerminal,
    name: "/traceway",
    description:
      "Installs the traceway CLI, then uses it: exceptions, logs, endpoints, and metrics, from bug report to root cause.",
    tags: ["CLI install", "Telemetry queries", "Debugging", "Issue triage"],
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
          <div className="flex flex-col items-center text-center">
            <Chip>
              <Bot className="h-3 w-3 inline mr-1" />
              Agent Skills
            </Chip>
            <h1 className="mt-6 max-w-[30ch]">
              AI-first observability. <em>Your agent does the debugging.</em>
            </h1>
            <p className="hero-sub text-pretty">
              Your agent sets up Traceway, queries production telemetry, and
              finds the root cause.
            </p>
            <div className="mt-8 w-full max-w-[640px]">
              <SkillInstallCommand size="lg" className="w-full" />
            </div>
            <p className="dim mt-5 font-mono text-[0.75rem]">
              Works with Claude Code, Cursor, Codex, and any agent that reads
              SKILL.md.
            </p>
            <p className="mt-3 font-mono text-[0.6875rem] uppercase tracking-[0.08em]">
              <Link
                href={GITHUB_URL}
                target="_blank"
                rel="noopener noreferrer"
                className="text-fg-2 hover:text-a2 transition-colors"
              >
                View on GitHub →
              </Link>
            </p>
          </div>
        </div>
      </section>

      <section className="wrap pt-8 pb-24">
        <SectionHead
          eyebrow="The skills"
          title={
            <>
              Two skills, <em>one install.</em>
            </>
          }
          description="Plain Markdown playbooks your agent loads on demand. One sets up your project, the other queries and debugs it."
        />
        <dl className="grid gap-5 md:grid-cols-2">
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
              <dd className="mt-4">
                <p className="muted text-sm leading-relaxed">
                  {skill.description}
                </p>
                <div className="mt-5 flex flex-wrap gap-2">
                  {skill.tags.map((tag) => (
                    <span key={tag} className="tag">
                      {tag}
                    </span>
                  ))}
                </div>
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
                Paste the bug report as written. The agent pulls the matching
                exceptions, the logs around the failure, and endpoint stats,
                then opens the failing code.
              </p>
              <ul className="feat-bullets">
                <li>Grouped exceptions with full stack traces</li>
                <li>Logs correlated by trace id</li>
                <li>First-seen times point at the breaking deploy</li>
                <li>Ends with a proposed fix in your code</li>
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
                Built for non-interactive use: JSON output, stable errors, and
                nothing that hangs waiting for input. Humans on a TTY still
                get tables.
              </p>
              <ul className="feat-bullets">
                <li>JSON when piped, tables on a TTY</li>
                <li>--fields trims responses to what was asked</li>
                <li>Stable error identifiers and exit codes</li>
                <li>Mutations require --yes, nothing hangs</li>
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
            description="/traceway-setup picks the right path for each part of the stack: OTel for backends, Traceway SDKs for browser and mobile. Then it verifies data is actually arriving."
            bullets={[
              "Detects frameworks and services from the repo",
              "OTel for backends, SDKs for web and mobile",
              "Covers background tasks, AI traces, and source maps",
              "Verifies grouped endpoints in the dashboard",
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
                  a: "A skill is a Markdown playbook (SKILL.md) that compatible coding agents load when a task matches its description. Traceway ships two: one that instruments a project, and one that operates Traceway from the terminal, from installing the CLI to querying telemetry and investigating bugs. They are versioned in the open-source repo like any other code.",
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
                  a: "No. The /traceway skill installs it: it downloads the right binary for the platform, authenticates against your instance, and selects a project before running its first query.",
                },
              ]}
            />
          </div>
        </div>
      </section>
    </main>
  );
}
