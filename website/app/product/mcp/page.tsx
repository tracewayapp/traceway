import Link from "next/link";
import type { Metadata } from "next";
import { Plug, ShieldCheck, Boxes, Lock } from "lucide-react";

import { Chip } from "@/components/chip";
import { SectionHead } from "@/components/section-head";
import { FaqList } from "@/components/faq-list";
import { FinalCTA } from "@/components/final-cta";
import { AuroraBackground } from "@/components/aurora-background";
import { Eyebrow } from "@/components/eyebrow";
import { IsoDiagram } from "@/components/iso-diagram";
import { Terminal } from "@/components/terminal";
import { CopyCommand } from "@/components/copy-command";
import { GITHUB_URL } from "@/lib/links";

export const metadata: Metadata = {
  title: "MCP Server · Traceway",
  description:
    "Traceway speaks the Model Context Protocol. Point Claude Code, Claude Desktop, Cursor, or any MCP client at your instance and it investigates your production exceptions, logs, latency, and metrics, with OAuth login built in.",
};

const CLIENTS = ["Claude Code", "Claude Desktop", "Cursor", "MCP Inspector"];

const TOOL_GROUPS = [
  {
    label: "Errors",
    tools: [
      "list_exceptions",
      "get_exception",
      "get_exception_occurrence",
      "archive_exceptions",
      "unarchive_exceptions",
    ],
  },
  {
    label: "Performance",
    tools: [
      "list_endpoints",
      "endpoints_chart",
      "get_endpoint_request",
      "get_slow_endpoint_config",
    ],
  },
  {
    label: "Signals",
    tools: [
      "query_logs",
      "query_metrics",
      "get_task",
      "get_ai_trace",
      "get_session",
      "get_trace",
    ],
  },
  {
    label: "Projects",
    tools: ["list_projects"],
  },
];

export default function McpPage() {
  return (
    <main className="relative">
      <section className="hero hero-product relative">
        <AuroraBackground variant="hero" />
        <div className="wrap relative z-10">
          <div className="flex flex-col items-center text-center">
            <Chip>
              <Plug className="h-3 w-3 inline mr-1" />
              MCP Server
            </Chip>
            <h1 className="mt-6 max-w-[26ch]">
              Production context, <em>over MCP.</em>
            </h1>
            <p className="hero-sub text-pretty">
              Point any MCP client at your Traceway instance. It logs in over
              OAuth and starts querying your exceptions, logs, latency, and
              metrics, directly.
            </p>
            <div className="mt-8 w-full max-w-[640px]">
              <CopyCommand
                size="lg"
                className="w-full"
                command="claude mcp add --transport http traceway https://cloud.tracewayapp.com/mcp"
              />
            </div>
            <p className="dim mt-5 font-mono text-[0.75rem]">
              Works with Claude Code, Claude Desktop, Cursor, and any MCP client.
            </p>
            <p className="mt-3 font-mono text-[0.6875rem] uppercase tracking-[0.08em]">
              <Link
                href="https://docs.tracewayapp.com/learn/mcp"
                target="_blank"
                rel="noopener noreferrer"
                className="text-fg-2 hover:text-a2 transition-colors"
              >
                Read the MCP docs →
              </Link>
            </p>
          </div>
        </div>
      </section>

      <section className="wrap relative z-10 pb-16 -mt-2">
        <IsoDiagram
          src="/diagrams/mcp-flow.svg"
          alt="Traceway MCP architecture: MCP clients authenticate over OAuth and query production errors, performance, and signals through the /mcp endpoint, backed by ClickHouse and Postgres."
          maxWidth={1040}
          priority
        />
      </section>

      <div className="band-light">
        <section className="wrap">
          <div className="feature-row">
            <div className="feat-copy">
              <Eyebrow>Remote</Eyebrow>
              <h2>
                Zero install. <em>Login built in.</em>
              </h2>
              <p>
                Every Traceway backend serves the tools over streamable HTTP at{" "}
                <code>/mcp</code>. On first connect the client registers itself,
                opens your browser on the instance&apos;s consent page, and holds
                a short-lived access token plus a rotating refresh token. Nothing
                to install or configure on the host.
              </p>
              <ul className="feat-bullets">
                <li>Dynamic client registration (RFC 7591)</li>
                <li>Browser consent on your own instance</li>
                <li>Short-lived access tokens, rotating refresh</li>
                <li>Personal access token fallback for CI</li>
              </ul>
            </div>
            <Terminal
              title="bash · remote MCP"
              lines={[
                {
                  ln: "1",
                  type: "tx",
                  content: (
                    <>
                      <span className="cmd">$</span> claude mcp add --transport
                      http traceway https://cloud.tracewayapp.com/mcp
                    </>
                  ),
                },
                {
                  ln: "2",
                  type: "mute",
                  content: "# opens your browser on the consent page…",
                },
                {
                  ln: "3",
                  type: "ok",
                  content: "✓ traceway connected (OAuth, no PAT to paste)",
                },
                {
                  ln: "4",
                  type: "tx",
                  content: (
                    <>
                      <span className="cmd">›</span> what&apos;s broken in prod
                      right now?
                    </>
                  ),
                },
                {
                  ln: "5",
                  type: "mute",
                  content:
                    "# → list_exceptions, query_logs, list_endpoints (last 1h)",
                },
              ]}
            />
          </div>
        </section>

        <section className="wrap">
          <div className="feature-row reverse">
            <div className="feat-copy">
              <Eyebrow>Local</Eyebrow>
              <h2>
                Or run it <em>on stdio</em>
              </h2>
              <p>
                The <code>traceway</code> CLI doubles as a local MCP server. Log
                in once in a terminal, pick a default project, and connect over
                stdio. It reuses your CLI session and refreshes device-login
                tokens automatically.
              </p>
              <ul className="feat-bullets">
                <li>One binary, no extra daemon</li>
                <li>Reuses your CLI login and current project</li>
                <li>Headless via TRACEWAY_URL + TRACEWAY_TOKEN</li>
                <li>Same tools as the remote server</li>
              </ul>
            </div>
            <Terminal
              title="bash · local MCP"
              lines={[
                {
                  ln: "1",
                  type: "tx",
                  content: (
                    <>
                      <span className="cmd">$</span> traceway login --url
                      https://your-instance
                    </>
                  ),
                },
                {
                  ln: "2",
                  type: "tx",
                  content: (
                    <>
                      <span className="cmd">$</span> traceway projects use
                      checkout-api
                    </>
                  ),
                },
                {
                  ln: "3",
                  type: "tx",
                  content: (
                    <>
                      <span className="cmd">$</span> claude mcp add traceway --
                      traceway mcp
                    </>
                  ),
                },
                {
                  ln: "4",
                  type: "ok",
                  content: "✓ traceway mcp serving on stdio",
                },
              ]}
            />
          </div>
        </section>

        <section className="wrap py-8">
          <Eyebrow>The tools</Eyebrow>
          <h2 className="mt-4 max-w-[22ch]">
            Sixteen tools. <em>Read-only by default.</em>
          </h2>
          <p className="muted mt-4 max-w-[640px] text-pretty">
            Each tool wraps one API call and declares an output schema, so
            results come back as validated structured content. Only the two
            archive tools mutate anything, and only when you ask for it by name.
          </p>

          <dl className="mt-14 grid gap-6 sm:grid-cols-2 lg:grid-cols-4">
            {TOOL_GROUPS.map((group) => (
              <div
                key={group.label}
                className="rounded-2xl p-6"
                style={{
                  background: "var(--ink-0)",
                  border: "1px solid var(--hair-2)",
                  boxShadow:
                    "0 1px 2px rgba(10,14,24,0.04), 0 12px 30px -18px rgba(10,14,24,0.14)",
                }}
              >
                <dt>
                  <p className="font-mono text-[0.6875rem] uppercase tracking-[0.14em] text-fg-3">
                    {group.label}
                  </p>
                </dt>
                <dd className="m-0 mt-4">
                  <ul className="flex flex-col gap-2">
                    {group.tools.map((tool) => (
                      <li
                        key={tool}
                        className="font-mono text-[0.8125rem] text-fg-1"
                      >
                        {tool}
                      </li>
                    ))}
                  </ul>
                </dd>
              </div>
            ))}
          </dl>

          <div className="mt-14 grid gap-6 md:grid-cols-3">
            <FeatureCard
              icon={<ShieldCheck className="h-5 w-5" />}
              title="Safe by default"
              body="Every read tool is annotated read-only. Archiving needs an explicit request; investigating an error never touches it."
            />
            <FeatureCard
              icon={<Boxes className="h-5 w-5" />}
              title="Prompts and resources"
              body="The same investigation playbooks the Traceway skill ships, exposed as MCP prompts (debug_issue, investigate_performance) and knowledge resources."
            />
            <FeatureCard
              icon={<Lock className="h-5 w-5" />}
              title="Your data, your instance"
              body="Self-hosted or cloud, the server runs where your telemetry lives. Tokens are scoped to the user who approved the connection."
            />
          </div>
        </section>

        <section className="wrap py-16 text-center">
          <div className="max-w-3xl mx-auto flex flex-col items-center gap-5">
            <Eyebrow>Compatibility</Eyebrow>
            <h2>One protocol, every client</h2>
            <p style={{ color: "var(--fg-1)", fontSize: 17 }}>
              Traceway implements the open Model Context Protocol: streamable
              HTTP with OAuth for remote, stdio for local. Any spec-compliant
              client can connect, no plugin required.
            </p>
            <div className="flex flex-wrap items-center justify-center gap-2.5 pt-2">
              {CLIENTS.map((client) => (
                <span key={client} className="tag">
                  {client}
                </span>
              ))}
            </div>
            <p className="dim mt-2 text-sm">
              Prefer skills over MCP tools?{" "}
              <Link
                href="/product/agent-skills"
                className="text-a2 hover:underline"
              >
                See Agent Skills →
              </Link>
            </p>
          </div>
        </section>
      </div>

      <FinalCTA
        title={
          <>
            Give your agent <em>a live connection.</em>
          </>
        }
        description="Connect an MCP client to your instance and let it investigate the next issue."
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

      <section className="wrap pt-10 pb-24">
        <div className="max-w-3xl mx-auto">
          <SectionHead
            align="center"
            eyebrow="FAQ"
            title="Questions about the MCP server"
          />
          <div className="mt-4">
            <FaqList
              items={[
                {
                  q: "What is the difference between MCP and the agent skills?",
                  a: (
                    <>
                      <p>
                        The MCP server gives an agent live tools and a login to
                        your instance. The agent skills are Markdown playbooks
                        that drive the <code>traceway</code> CLI. They share the
                        same knowledge; pick whichever your client supports.
                      </p>
                      <p>
                        <Link
                          href="/product/agent-skills"
                          className="text-a2 hover:underline"
                        >
                          See Agent Skills →
                        </Link>
                      </p>
                    </>
                  ),
                },
                {
                  q: "Do I have to paste an API token?",
                  a: "No. The remote server handles OAuth for you: the client registers itself and you approve it in the browser. CI and non-OAuth clients can send a personal access token on the Authorization header instead.",
                },
                {
                  q: "Can an agent change my production data?",
                  a: "Only exception groups can be archived, and only with the archive tools, which are marked mutating and instruct agents to call them solely on explicit request. Everything else is read-only.",
                },
                {
                  q: "Does it work with self-hosted Traceway?",
                  a: "Yes. Every backend serves /mcp, and the OAuth issuer is derived from the request, so it works behind a reverse proxy with no extra configuration.",
                },
                {
                  q: "Which clients are supported?",
                  a: "Any MCP client that speaks streamable HTTP (remote) or stdio (local): Claude Code, Claude Desktop, Cursor, MCP Inspector, and more.",
                },
              ]}
            />
          </div>
        </div>
      </section>
    </main>
  );
}

function FeatureCard({
  icon,
  title,
  body,
}: {
  icon: React.ReactNode;
  title: string;
  body: string;
}) {
  return (
    <div
      className="rounded-2xl p-6"
      style={{
        background: "var(--ink-0)",
        border: "1px solid var(--hair-2)",
        boxShadow:
          "0 1px 2px rgba(10,14,24,0.04), 0 12px 30px -18px rgba(10,14,24,0.14)",
      }}
    >
      <div className="text-a2">{icon}</div>
      <h3 className="mt-4 text-[1.0625rem] font-semibold text-fg-0">{title}</h3>
      <p className="muted mt-2 text-[0.9375rem] text-pretty">{body}</p>
    </div>
  );
}
