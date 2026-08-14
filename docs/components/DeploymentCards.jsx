import Link from "next/link";

const DEPLOYMENTS = [
  {
    title: "All-in-One Container",
    badge: "Recommended",
    href: "/server/all-in-one",
    subtitle: "ClickHouse + PostgreSQL + Backend + Frontend",
    description:
      "Everything in a single Docker container managed by supervisord. No external dependencies needed.",
  },
  {
    title: "Docker Compose",
    href: "/server/docker-compose",
    subtitle: "Backend + Frontend + ClickHouse + PostgreSQL",
    description:
      "Each component runs as its own container, orchestrated with a single command.",
  },
  {
    title: "Minimal Container",
    href: "/server/minimal",
    subtitle: "Backend + Frontend only",
    description:
      "Lightweight Alpine image (~20-30MB). Connect to your own external ClickHouse and PostgreSQL.",
  },
  {
    title: "SQLite",
    href: "/server/sqlite",
    subtitle: "Backend + Frontend + SQLite",
    description:
      "Single Alpine container with embedded SQLite. No external databases needed. Optional S3 for source maps and recordings.",
  },
  {
    title: "DuckDB",
    href: "/server/duckdb",
    subtitle: "Backend + Frontend + SQLite + DuckDB",
    description:
      "Single Debian container with a columnar telemetry store. Same zero-dependency setup as SQLite with far more dashboard headroom.",
  },
  {
    title: "Browser (Monitors)",
    href: "/server/browser",
    subtitle: "DuckDB + Playwright + Chromium",
    description:
      "The DuckDB container plus headless Chromium, so Playwright browser monitors run in-process. HTTP/TCP monitors work in every image.",
  },
  {
    title: "Local Setup",
    href: "/server/local-setup",
    subtitle: "Backend + Frontend from source",
    description:
      "Development setup with Go and Node.js. Requires local ClickHouse and PostgreSQL.",
  },
  {
    title: "Haloy",
    href: "/server/haloy",
    subtitle: "ClickHouse + PostgreSQL + Backend + Frontend",
    description:
      "Deploy to your own server with automatic SSL and a single YAML config using the Haloy CLI.",
  }
];

export function DeploymentCards() {
  return (
    <div className="deployment-grid">
      {DEPLOYMENTS.map((d) => (
        <Link key={d.href} href={d.href} className="deployment-card">
          <div className="deployment-card-top">
            <span className="deployment-card-title">{d.title}</span>
            {d.badge && (
              <span className="deployment-card-badge">{d.badge}</span>
            )}
          </div>
          <span className="deployment-card-subtitle">{d.subtitle}</span>
          <span className="deployment-card-desc">{d.description}</span>
        </Link>
      ))}
    </div>
  );
}
