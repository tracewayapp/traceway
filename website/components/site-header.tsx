"use client";

import { useState, useRef, useEffect, useCallback } from "react";
import Link from "next/link";
import Image from "next/image";
import {
  Github,
  ChevronDown,
  ScrollText,
  Network,
  BarChart3,
  Video,
  Bug,
  Workflow,
  Activity,
  Smartphone,
  Bot,
  Braces,
  Plug,
  LayoutDashboard,
} from "lucide-react";
import { MobileNav } from "@/components/mobile-nav";
import { DiscordIcon } from "@/components/discord-icon";
import { cn } from "@/lib/utils";
import { GITHUB_URL, DISCORD_URL } from "@/lib/links";

type NavItem = {
  title: string;
  description: string;
  href: string;
  icon: typeof ScrollText;
};

const PILLARS: NavItem[] = [
  {
    title: "Logs",
    description: "Search every log, linked to its trace.",
    href: "/product/logs",
    icon: ScrollText,
  },
  {
    title: "Traces",
    description: "Follow a request across every service.",
    href: "/product/traces",
    icon: Network,
  },
  {
    title: "Metrics",
    description: "Application and server metrics with dashboards.",
    href: "/product/metrics",
    icon: BarChart3,
  },
  {
    title: "Session Replay",
    description: "Watch what the user did before every error.",
    href: "/product/session-replay",
    icon: Video,
  },
  {
    title: "Exceptions / Stack Traces",
    description:
      "Grouped, normalized, and paired with the replay that caused them.",
    href: "/product/stack-traces",
    icon: Bug,
  },
];

const SPECIALIZED: NavItem[] = [
  {
    title: "Agent Skills",
    description: "Your AI agent debugs with Traceway.",
    href: "/product/agent-skills",
    icon: Bot,
  },
  {
    title: "MCP Server",
    description: "Connect any MCP client to your instance.",
    href: "/product/mcp",
    icon: Plug,
  },
  {
    title: "AI Tracing",
    description: "LLM cost, tokens, latency, conversations.",
    href: "/product/ai-tracing",
    icon: Workflow,
  },
  {
    title: "Dashboards as Code",
    description: "JSON dashboards you version, sync, and share.",
    href: "/product/dashboards",
    icon: LayoutDashboard,
  },
  {
    title: "Performance",
    description: "P50/P95/P99 percentiles, waterfall traces.",
    href: "/product/performance",
    icon: Activity,
  },
  {
    title: "Flutter Session Replay",
    description: "Open-source mobile replay, 10s before every exception.",
    href: "/product/flutter-session-replay",
    icon: Smartphone,
  },
  {
    title: "Symbolicator",
    description: "Minified JS and Dart stack traces, resolved to your source.",
    href: "/product/symbolication",
    icon: Braces,
  },
];

export function SiteHeader() {
  const [open, setOpen] = useState(false);
  const timeoutRef = useRef<NodeJS.Timeout | null>(null);
  const dropdownRef = useRef<HTMLDivElement>(null);

  const handleEnter = useCallback(() => {
    if (timeoutRef.current) clearTimeout(timeoutRef.current);
    setOpen(true);
  }, []);

  const handleLeave = useCallback(() => {
    timeoutRef.current = setTimeout(() => setOpen(false), 150);
  }, []);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") setOpen(false);
    };
    const onClick = (e: MouseEvent) => {
      if (!dropdownRef.current) return;
      if (!dropdownRef.current.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener("keydown", onKey);
    document.addEventListener("mousedown", onClick);
    return () => {
      document.removeEventListener("keydown", onKey);
      document.removeEventListener("mousedown", onClick);
      if (timeoutRef.current) clearTimeout(timeoutRef.current);
    };
  }, []);

  return (
    <nav
      className="site-nav sticky top-0 z-50 border-b transition-colors"
      style={{
        borderColor: "var(--hair)",
        background: "var(--ink-0)",
      }}
    >
      <div className="wrap flex h-16 items-center justify-between">
        <div className="flex items-center gap-6">
          <Link
            href="/"
            className="flex items-center gap-2"
            aria-label="Traceway home"
          >
            <Image
              src="/images/logo.png"
              alt="Traceway"
              width={120}
              height={32}
              className="logo-img h-7 w-auto"
              priority
            />
          </Link>

          <div className="hidden md:flex items-center gap-1">
            <div
              ref={dropdownRef}
              className="relative"
              onMouseEnter={handleEnter}
              onMouseLeave={handleLeave}
            >
              <button
                className={cn(
                  "inline-flex items-center gap-1.5 h-9 px-3 rounded-md text-[14px] font-medium transition-colors",
                  "text-[color:var(--fg-1)] hover:text-[color:var(--fg-0)] hover:bg-[color:var(--ink-2)]",
                  open && "text-[color:var(--fg-0)] bg-[color:var(--ink-2)]",
                )}
                style={{ fontFamily: "var(--font-display)" }}
                aria-expanded={open}
                aria-haspopup="menu"
              >
                Solutions
                <ChevronDown
                  className={cn(
                    "size-3 transition-transform opacity-60",
                    open && "rotate-180",
                  )}
                />
              </button>

              <div
                className={cn(
                  "absolute top-full left-0 mt-2 w-[560px] max-w-[calc(100vw-1.5rem)] overflow-hidden rounded-[14px] lg:w-[860px]",
                  "transition-all duration-150",
                  open
                    ? "opacity-100 translate-y-0 pointer-events-auto"
                    : "opacity-0 -translate-y-1 pointer-events-none",
                )}
                style={{
                  background: "var(--ink-2)",
                  border: "1px solid var(--hair-2)",
                  boxShadow: "0 24px 50px -28px rgba(0,0,0,0.8)",
                }}
                role="menu"
              >
                <div className="grid grid-cols-2 gap-9 p-6 lg:grid-cols-[1fr_1fr_300px]">
                  <NavColumn
                    eyebrow="Platform"
                    items={PILLARS}
                    onNavigate={() => setOpen(false)}
                  />
                  <NavColumn
                    eyebrow="Specialized"
                    items={SPECIALIZED}
                    onNavigate={() => setOpen(false)}
                  />
                  <Link
                    href="/product/agent-skills"
                    onClick={() => setOpen(false)}
                    role="menuitem"
                    className="group hidden overflow-hidden rounded-xl transition-colors lg:block"
                    style={{
                      background: "var(--ink-1)",
                      border: "1px solid var(--hair)",
                    }}
                  >
                    {/* eslint-disable-next-line @next/next/no-img-element */}
                    <img
                      src="/images/agent-skills-feature.jpg"
                      alt=""
                      className="block w-full"
                      style={{
                        height: 150,
                        objectFit: "cover",
                        objectPosition: "center top",
                      }}
                    />
                    <div className="p-4">
                      <MenuEyebrow>Featured</MenuEyebrow>
                      <div
                        className="mt-2 text-[15px] font-semibold"
                        style={{
                          fontFamily: "var(--font-body)",
                          color: "var(--fg-0)",
                          letterSpacing: "-0.01em",
                        }}
                      >
                        Agent Skills
                      </div>
                      <p
                        className="mt-1 text-[13px] leading-snug"
                        style={{
                          fontFamily: "var(--font-body)",
                          color: "var(--fg-2)",
                        }}
                      >
                        Your coding agent sets up Traceway and debugs production
                        for you.
                      </p>
                    </div>
                  </Link>
                </div>

                <div
                  className="flex items-center justify-between gap-4 px-6 py-4"
                  style={{
                    borderTop: "1px solid var(--hair)",
                    background: "var(--ink-1)",
                  }}
                >
                  <div>
                    <MenuEyebrow>Open source</MenuEyebrow>
                    <p
                      className="mt-1 text-[13px]"
                      style={{
                        fontFamily: "var(--font-body)",
                        color: "var(--fg-1)",
                      }}
                    >
                      MIT-licensed. Self-host in minutes or start free in the
                      cloud.
                    </p>
                  </div>
                  <Link
                    href={GITHUB_URL}
                    target="_blank"
                    rel="noopener noreferrer"
                    onClick={() => setOpen(false)}
                    className="btn btn-accent btn-sm shrink-0"
                  >
                    <Github className="h-4 w-4" />
                    Star on GitHub
                  </Link>
                </div>
              </div>
            </div>

            <Link
              href="/cloud"
              className="h-9 px-3 rounded-md inline-flex items-center text-[14px] font-medium transition-colors text-[color:var(--fg-1)] hover:text-[color:var(--fg-0)] hover:bg-[color:var(--ink-2)]"
              style={{ fontFamily: "var(--font-display)" }}
            >
              Cloud
            </Link>
            <Link
              href="/blog"
              className="h-9 px-3 rounded-md inline-flex items-center text-[14px] font-medium transition-colors text-[color:var(--fg-1)] hover:text-[color:var(--fg-0)] hover:bg-[color:var(--ink-2)]"
              style={{ fontFamily: "var(--font-display)" }}
            >
              Blog
            </Link>
            <Link
              href="https://docs.tracewayapp.com"
              target="_blank"
              rel="noopener noreferrer"
              className="h-9 px-3 rounded-md inline-flex items-center text-[14px] font-medium transition-colors text-[color:var(--fg-1)] hover:text-[color:var(--fg-0)] hover:bg-[color:var(--ink-2)]"
              style={{ fontFamily: "var(--font-display)" }}
            >
              Docs
            </Link>
          </div>
        </div>

        <div className="hidden md:flex items-center gap-3">
          <Link
            href={GITHUB_URL}
            target="_blank"
            rel="noopener noreferrer"
            className="inline-flex items-center justify-center h-8 w-8 rounded-md text-[color:var(--fg-2)] hover:text-[color:var(--fg-0)] hover:bg-[color:var(--ink-2)] transition-colors"
          >
            <Github className="h-4 w-4" />
            <span className="sr-only">GitHub</span>
          </Link>
          <Link
            href={DISCORD_URL}
            target="_blank"
            rel="noopener noreferrer"
            className="inline-flex items-center justify-center h-8 w-8 rounded-md text-[color:var(--fg-2)] hover:text-[color:var(--fg-0)] hover:bg-[color:var(--ink-2)] transition-colors"
          >
            <DiscordIcon className="h-4 w-4" />
            <span className="sr-only">Discord</span>
          </Link>
          <Link
            href="https://cloud.tracewayapp.com/login"
            className="h-9 px-4 inline-flex items-center text-[14px] font-medium rounded-md transition-colors text-[color:var(--fg-1)] hover:text-[color:var(--fg-0)] hover:bg-[color:var(--ink-2)]"
            style={{ fontFamily: "var(--font-display)" }}
          >
            Sign in
          </Link>
          <Link
            href="https://cloud.tracewayapp.com/register"
            className="btn btn-accent btn-sm"
          >
            Start for free
          </Link>
        </div>

        <MobileNav pillars={PILLARS} specialized={SPECIALIZED} />
      </div>
    </nav>
  );
}

function MenuEyebrow({ children }: { children: React.ReactNode }) {
  return (
    <div
      className="text-[13.5px] font-medium uppercase tracking-[0.16em]"
      style={{ fontFamily: "var(--font-mono)", color: "var(--fg-3)" }}
    >
      {children}
    </div>
  );
}

function NavColumn({
  eyebrow,
  items,
  onNavigate,
}: {
  eyebrow: string;
  items: NavItem[];
  onNavigate?: () => void;
}) {
  return (
    <div>
      <MenuEyebrow>{eyebrow}</MenuEyebrow>
      <div className="mt-5 flex flex-col">
        {items.map((item) => (
          <Link
            key={item.href}
            href={item.href}
            onClick={onNavigate}
            role="menuitem"
            className="-mx-2 block whitespace-nowrap rounded-md px-2 py-[9px] text-[15px] font-medium transition-colors text-[color:var(--fg-1)] hover:text-[color:var(--fg-0)] hover:bg-[color:rgba(255,255,255,0.04)]"
            style={{ fontFamily: "var(--font-body)", letterSpacing: "-0.01em" }}
          >
            {item.title}
          </Link>
        ))}
      </div>
    </div>
  );
}
