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
    description: "Grouped, normalized, and paired with the replay that caused them.",
    href: "/product/stack-traces",
    icon: Bug,
  },
];

const SPECIALIZED: NavItem[] = [
  {
    title: "AI Tracing",
    description: "LLM cost, tokens, latency, conversations.",
    href: "/product/ai-tracing",
    icon: Workflow,
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
          <Link href="/" className="flex items-center gap-2" aria-label="Traceway home">
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
                  open && "text-[color:var(--fg-0)] bg-[color:var(--ink-2)]"
                )}
                style={{ fontFamily: "var(--font-display)" }}
                aria-expanded={open}
                aria-haspopup="menu"
              >
                Product
                <ChevronDown
                  className={cn("size-3 transition-transform opacity-60", open && "rotate-180")}
                />
              </button>

              <div
                className={cn(
                  "absolute top-full left-0 mt-2 pt-0 min-w-[720px] rounded-[12px]",
                  "transition-all duration-150",
                  open ? "opacity-100 translate-y-0 pointer-events-auto" : "opacity-0 -translate-y-1 pointer-events-none"
                )}
                style={{
                  background: "var(--ink-2)",
                  border: "1px solid var(--hair-2)",
                  boxShadow: "0 16px 40px -24px rgba(0,0,0,0.7)",
                  padding: 24,
                }}
                role="menu"
              >
                <div className="grid grid-cols-2 gap-8">
                  <div>
                    <div
                      className="eyebrow"
                      style={{ fontSize: 10.5, color: "var(--fg-3)", marginBottom: 14 }}
                    >
                      Observability pillars
                    </div>
                    <div className="flex flex-col">
                      {PILLARS.map((p) => (
                        <MegaLink key={p.href} item={p} onClick={() => setOpen(false)} />
                      ))}
                    </div>
                  </div>
                  <div>
                    <div
                      className="eyebrow"
                      style={{ fontSize: 10.5, color: "var(--fg-3)", marginBottom: 14 }}
                    >
                      Specialized
                    </div>
                    <div className="flex flex-col">
                      {SPECIALIZED.map((p) => (
                        <MegaLink key={p.href} item={p} onClick={() => setOpen(false)} />
                      ))}
                    </div>
                  </div>
                </div>

                <div
                  className="mt-4 pt-4 flex items-center justify-between"
                  style={{ borderTop: "1px solid var(--hair)" }}
                >
                  <Link
                    href="https://docs.tracewayapp.com"
                    target="_blank"
                    rel="noopener noreferrer"
                    className="text-[11px] tracking-[.08em] uppercase hover:text-[color:var(--a2)] transition-colors"
                    style={{ fontFamily: "var(--font-mono)", color: "var(--fg-2)" }}
                  >
                    Documentation →
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
              href="/blog/engineering"
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

function MegaLink({
  item,
  onClick,
}: {
  item: NavItem;
  onClick?: () => void;
}) {
  const Icon = item.icon;
  return (
    <Link
      href={item.href}
      onClick={onClick}
      className="group grid grid-cols-[32px_1fr] gap-3 items-start py-2 px-2.5 -mx-2.5 rounded-md transition-colors hover:bg-[color:rgba(255,255,255,0.04)]"
    >
      <div
        className="h-8 w-8 rounded-md grid place-items-center"
        style={{
          background: "rgba(255,255,255,0.04)",
          border: "1px solid var(--hair)",
          color: "var(--a2)",
        }}
      >
        <Icon className="h-4 w-4" />
      </div>
      <div>
        <div
          className="text-[14px] font-medium leading-tight"
          style={{ fontFamily: "var(--font-display)", color: "var(--fg-0)", letterSpacing: "-0.01em" }}
        >
          {item.title}
        </div>
        <div
          className="text-[12px] mt-0.5 leading-snug"
          style={{ color: "var(--fg-3)", fontFamily: "var(--font-mono)" }}
        >
          {item.description}
        </div>
      </div>
    </Link>
  );
}
