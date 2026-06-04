import Link from "next/link";
import Image from "next/image";
import { Github } from "lucide-react";
import { DiscordIcon } from "@/components/discord-icon";
import { GITHUB_URL, DISCORD_URL } from "@/lib/links";

type Column = {
  heading: string;
  links: { label: string; href: string; external?: boolean }[];
};

const COLUMNS: Column[] = [
  {
    heading: "Pillars",
    links: [
      { label: "Logs", href: "/product/logs" },
      { label: "Traces", href: "/product/traces" },
      { label: "Metrics", href: "/product/metrics" },
      { label: "Session Replay", href: "/product/session-replay" },
      { label: "Exceptions / Stack Traces", href: "/product/stack-traces" },
    ],
  },
  {
    heading: "Specialized",
    links: [
      { label: "AI Tracing", href: "/product/ai-tracing" },
      { label: "Performance", href: "/product/performance" },
      { label: "Flutter Session Replay", href: "/product/flutter-session-replay" },
    ],
  },
  {
    heading: "Resources",
    links: [
      { label: "Blog", href: "/blog/engineering" },
      { label: "Release Notes", href: "/blog" },
      { label: "Docs", href: "https://docs.tracewayapp.com", external: true },
      { label: "GitHub", href: GITHUB_URL, external: true },
      { label: "Discord", href: DISCORD_URL, external: true },
    ],
  },
  {
    heading: "Hosting",
    links: [
      { label: "Cloud", href: "/cloud" },
      { label: "Self-host", href: "https://docs.tracewayapp.com", external: true },
    ],
  },
  {
    heading: "About",
    links: [
      { label: "Privacy Policy", href: "/privacy-policy" },
      { label: "Terms of Use", href: "/terms-of-use" },
      { label: "Contact", href: "/contact" },
    ],
  },
];

export function SiteFooter() {
  const year = new Date().getFullYear();
  return (
    <footer
      className="mt-20"
      style={{ borderTop: "1px solid var(--hair)", padding: "50px 0 40px" }}
    >
      <div className="wrap">
        <div className="footer-grid grid gap-10 md:gap-9 grid-cols-2 md:grid-cols-[1.2fr_repeat(5,1fr)]">
          <div className="col-span-2 md:col-span-1">
            <Link href="/" className="inline-flex items-center gap-2">
              <Image
                src="/images/logo.png"
                alt="Traceway"
                width={120}
                height={28}
                className="logo-img h-6 w-auto"
              />
            </Link>
            <p
              className="mt-4 text-[13px] leading-relaxed max-w-xs"
              style={{ color: "var(--fg-2)", fontFamily: "var(--font-mono)" }}
            >
              Observability that tells you what to fix first. Open source. Self-host or cloud.
            </p>
            <div className="mt-5 flex items-center gap-2">
              <Link
                href={GITHUB_URL}
                target="_blank"
                rel="noopener noreferrer"
                className="h-8 w-8 inline-flex items-center justify-center rounded-md text-[color:var(--fg-2)] hover:text-[color:var(--fg-0)] hover:bg-[color:var(--ink-2)] transition-colors"
                aria-label="GitHub"
              >
                <Github className="h-4 w-4" />
              </Link>
              <Link
                href={DISCORD_URL}
                target="_blank"
                rel="noopener noreferrer"
                className="h-8 w-8 inline-flex items-center justify-center rounded-md text-[color:var(--fg-2)] hover:text-[color:var(--fg-0)] hover:bg-[color:var(--ink-2)] transition-colors"
                aria-label="Discord"
              >
                <DiscordIcon className="h-4 w-4" />
              </Link>
            </div>
          </div>

          {COLUMNS.map((col) => (
            <div key={col.heading}>
              <h5
                className="mb-3 text-[11px] uppercase tracking-[.1em] font-medium"
                style={{ color: "var(--fg-2)", fontFamily: "var(--font-mono)" }}
              >
                {col.heading}
              </h5>
              <ul className="flex flex-col gap-1.5">
                {col.links.map((l) => (
                  <li key={l.href}>
                    <Link
                      href={l.href}
                      {...(l.external ? { target: "_blank", rel: "noopener noreferrer" } : {})}
                      className="text-[13px] inline-block py-1 transition-colors hover:text-[color:var(--fg-0)]"
                      style={{ color: "var(--fg-1)" }}
                    >
                      {l.label}
                    </Link>
                  </li>
                ))}
              </ul>
            </div>
          ))}
        </div>

        <div
          className="mt-10 pt-6 flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3"
          style={{ borderTop: "1px solid var(--hair)" }}
        >
          <p
            className="text-[11px] uppercase tracking-[.08em]"
            style={{ color: "var(--fg-3)", fontFamily: "var(--font-mono)" }}
          >
            © {year} Traceway. All rights reserved.
          </p>
          <p
            className="text-[11px] uppercase tracking-[.08em]"
            style={{ color: "var(--fg-3)", fontFamily: "var(--font-mono)" }}
          >
            Built with OpenTelemetry · ClickHouse · Go
          </p>
        </div>
      </div>
    </footer>
  );
}
