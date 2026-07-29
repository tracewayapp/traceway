import Link from "next/link";
import { ArrowRight, LayoutDashboard } from "lucide-react";

import { Chip } from "@/components/chip";
import { SectionHead } from "@/components/section-head";
import { FeatureRow } from "@/components/feature-row";
import { FaqList } from "@/components/faq-list";
import { FinalCTA } from "@/components/final-cta";
import { AuroraBackground } from "@/components/aurora-background";

export default function DashboardsPage() {
  return (
    <main className="relative">
      <section className="hero hero-product relative">
        <AuroraBackground variant="hero" />
        <div className="wrap relative z-10">
          <Chip variant="ok">
            <LayoutDashboard className="h-3 w-3 inline mr-1" />
            Dashboards as Code
          </Chip>
          <h1 className="mt-6">
            Dashboards you can <em>diff, review, and ship.</em>
          </h1>
          <p className="hero-sub">
            Every Traceway dashboard is a plain JSON document underneath. Build
            it in the UI, export it to your repo, sync it from CI, and apply it
            across every project in your organization. No click-ops, no drift.
          </p>
          <div className="hero-cta-row">
            <Link
              href="https://docs.tracewayapp.com/learn/dashboards"
              className="btn btn-accent"
            >
              Read the Docs <ArrowRight className="h-4 w-4" />
            </Link>
            <Link
              href="https://cloud.tracewayapp.com/register"
              className="btn btn-ghost"
            >
              Try Traceway Cloud
            </Link>
          </div>
        </div>
      </section>

      {/* WHITE BAND: feature sections render on white */}
      <div className="band-light">
        <section className="wrap">
          <FeatureRow
            eyebrow="Plain JSON"
            title={
              <>
                Every dashboard is <em>one JSON document</em>
              </>
            }
            description="Widgets, sources, aggregations, and layout live in a single versioned document with a schemaVersion. Edit it in place from the dashboard menu, validated on save, or round-trip it through export and import. Widget ids are stable, so homepage stars survive edits."
            bullets={[
              "Edit as JSON directly in the UI, validated on save",
              "Export a dashboard, or your whole organization, as JSON",
              "Widget order in the array is the display order",
              "Stable widget ids keep starred widgets intact",
            ]}
            image={{
              src: "/images/dashboards-edit-json.png",
              alt: "Editing a Traceway dashboard as JSON",
            }}
          />
        </section>

        <section className="wrap">
          <FeatureRow
            reverse
            eyebrow="GitOps"
            title={
              <>
                Keep dashboards <em>in your repository</em>
              </>
            }
            description={
              <>
                The import API has an upsert mode that matches dashboards by
                name, so a CI job is one curl:{" "}
                <code>
                  POST /api/dashboards/import{" "}
                  {`{"mode": "upsert", "data": ...}`}
                </code>{" "}
                with a personal access token. Review dashboard changes in pull
                requests like any other code, and let the pipeline keep every
                environment in sync.
              </>
            }
            bullets={[
              "Upsert mode matches dashboards by name, so re-runs are safe",
              "Import a single dashboard or a whole-organization bundle",
              "Authenticate with a personal access token from CI",
              "The UI and the API edit the same document, so nothing drifts",
            ]}
            image={{
              src: "/images/dashboards-page.png",
              alt: "The Traceway dashboards page",
            }}
          />
        </section>

        <section className="wrap">
          <FeatureRow
            eyebrow="Shared across projects"
            title={
              <>
                Build once, <em>apply everywhere</em>
              </>
            }
            description="Dashboards belong to your organization, not to a single project. Apply the same dashboard to any set of projects; a fix made in one is instantly live in all of them. Each project still curates its own homepage by starring the widgets it cares about."
            bullets={[
              "One shared definition, per-project tabs",
              "Nothing is copied, and edits propagate everywhere",
              "Per-project starred widgets for the homepage",
              "Unapplying only removes it from that project's tabs",
            ]}
            image={{
              src: "/images/dashboards-apply.png",
              alt: "Applying a dashboard to projects",
            }}
          />
        </section>

        <section className="wrap">
          <FeatureRow
            reverse
            eyebrow="Grafana import"
            title={
              <>
                Bring the Grafana dashboards <em>you already have</em>
              </>
            }
            description="Paste any Grafana dashboard JSON export. Timeseries and graph panels become line charts, stat becomes a single value, gauges stay gauges, and Prometheus queries are unwrapped into metric sources with their label matchers and group-bys. Anything that cannot be converted faithfully is reported, not silently dropped."
            bullets={[
              "Panels and Prometheus queries converted automatically",
              "rate() approximations and skipped panels are called out",
              "Imported dashboards are normal dashboards you can edit freely",
            ]}
            image={{
              src: "/images/dashboards-grafana-warnings.png",
              alt: "Grafana import with conversion warnings",
            }}
          />
        </section>

        <section className="wrap">
          <FeatureRow
            eyebrow="Templates & ⌘K"
            title={
              <>
                Start from a template, <em>find anything fast</em>
              </>
            }
            description="One command palette covers your organization's dashboards, the built-in template marketplace, and every metric name seen across your projects. Install the OTelemetry Server Agent template for host metrics, Go Application for runtime metrics, or ClickHouse and DuckDB health dashboards for a monitored Traceway instance."
            bullets={[
              "⌘K searches dashboards, templates, and metrics together",
              "Templates install as normal dashboards you customize freely",
              "Selecting a metric opens the widget editor prefilled",
            ]}
            image={{
              src: "/images/dashboards-palette-templates.png",
              alt: "Dashboard templates in the command palette",
            }}
          />
        </section>
      </div>

      <FinalCTA
        title={
          <>
            Observability dashboards <em>without the click-ops</em>
          </>
        }
        description="Plain JSON, org-wide sharing, CI sync, Grafana import. Included on every plan."
        primary={{
          label: "Read the Dashboards docs",
          href: "https://docs.tracewayapp.com/learn/dashboards",
        }}
        secondary={{
          label: "Start Free",
          href: "https://cloud.tracewayapp.com/register",
        }}
      />

      <section className="wrap pt-10 pb-24">
        <div className="max-w-3xl mx-auto">
          <SectionHead
            align="center"
            eyebrow="FAQ"
            title="Questions about dashboards as code"
          />
          <div className="mt-4">
            <FaqList
              items={[
                {
                  q: "What does the dashboard JSON look like?",
                  a: (
                    <>
                      <p>
                        A dashboard is one document with a{" "}
                        <code>schemaVersion</code>, a name, and a{" "}
                        <code>widgets</code> array. Each widget carries its
                        type (line chart, area, stacked area, bar, single
                        value, gauge, table) and one or more sources: a metric
                        name, an aggregation, optional tag filters, and an
                        optional group-by that splits the chart into one
                        series per tag value. Array order is display order.
                      </p>
                    </>
                  ),
                },
                {
                  q: "How do I sync dashboards from CI?",
                  a: (
                    <>
                      <p>
                        Export the dashboard once, commit the JSON to your
                        repo, and have CI post it to{" "}
                        <code>/api/dashboards/import</code> with{" "}
                        <code>{`"mode": "upsert"`}</code> and a personal access
                        token. Upsert matches dashboards by name, so the job
                        idempotent. Re-running it converges every
                        environment to the file in your repository.
                      </p>
                    </>
                  ),
                },
                {
                  q: "What happens when someone edits a dashboard in the UI?",
                  a: "The UI edits the exact same JSON document the API serves. There is no separate representation. If you treat your repo as the source of truth, the next CI upsert brings the dashboard back to the committed state; if you prefer UI-first, just export after editing and commit the result.",
                },
                {
                  q: "How faithful is the Grafana import?",
                  a: "Panels and Prometheus queries are converted on a best-effort basis, and every compromise is reported in the import dialog: rate() approximations, dropped regex matchers, unsupported panel types. You end up with a normal Traceway dashboard you can keep editing, never a black box.",
                },
              ]}
            />
          </div>
        </div>
      </section>
    </main>
  );
}
