import Link from "next/link";
import type { Metadata } from "next";
import { ArrowRight, Boxes } from "lucide-react";

import { Chip } from "@/components/chip";
import { Eyebrow } from "@/components/eyebrow";
import { SectionHead } from "@/components/section-head";
import { FeatureRow } from "@/components/feature-row";
import { FaqList } from "@/components/faq-list";
import { FinalCTA } from "@/components/final-cta";
import { Terminal } from "@/components/terminal";
import { AuroraBackground } from "@/components/aurora-background";

export const metadata: Metadata = {
  title: "Fleet Overview · Traceway",
  description:
    "One page for every server, Kubernetes node, project, issue, monitor, and open on-call page across your organization. Cluster-wide instrumentation in one kubectl apply.",
};

export default function FleetPage() {
  return (
    <main className="relative">
      <section className="hero hero-product relative">
        <AuroraBackground variant="hero" />
        <div className="wrap relative z-10">
          <Chip variant="ok">
            <Boxes className="h-3 w-3 inline mr-1" />
            Organization Overview &amp; Kubernetes
          </Chip>
          <h1 className="mt-6">
            Ten projects. Sixty machines. <em>One page.</em>
          </h1>
          <p className="hero-sub">
            Per-project dashboards stop working the moment you have more than a
            few. The organization overview sits above all of them: every
            instance reporting in, every issue, every monitor, and every page
            that still needs a human. Kubernetes nodes group by cluster, plain
            hosts sit alongside them, and one click lands you in the dashboard
            for the box that is actually on fire.
          </p>
          <div className="hero-cta-row">
            <Link
              href="https://docs.tracewayapp.com/learn/organization-overview"
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

      <div className="band-light">
        <section className="wrap">
          <FeatureRow
            eyebrow="Operational pulse"
            title={
              <>
                The fleet, <em>as one heartbeat</em>
              </>
            }
            description="CPU, memory, disk, and network for every machine reporting into the organization, refreshed every minute. Rows sort worst-first, and stale outranks warning on purpose: a box that stopped talking is a bigger unknown than one running warm. Nothing to register and nothing to clean up, because an instance exists exactly as long as it reports."
            bullets={[
              "Critical, warning, stale, and healthy computed from live metrics",
              "One counter each for reporting, needs attention, stale, and network I/O",
              "Issues in 24h and open on-call pages across every project",
              "Search by host, OS, architecture, cloud region, cluster, or node",
            ]}
            image={{
              src: "/images/organization-overview.png",
              alt: "The Traceway organization overview: operational pulse counters and an instance table grouped by Kubernetes cluster",
            }}
          />
        </section>

        <section className="wrap">
          <div className="feature-row reverse">
            <div className="feat-copy">
              <Eyebrow>Kubernetes</Eyebrow>
              <h2>
                A whole cluster, <em>in one apply</em>
              </h2>
              <p>
                Two OpenTelemetry Collector workloads cover everything: a
                DaemonSet for node health, pod resource usage, and container
                logs, and one small Deployment for cluster state and Kubernetes
                events. Every node becomes an instance row, grouped under your
                cluster name. Half-migrated fleets still read as one fleet,
                because plain hosts sit in the same table.
              </p>
              <ul className="feat-bullets">
                <li>Node CPU, memory, disk, and network per node</li>
                <li>Pod metrics, container stdout, cluster events</li>
                <li>Group and filter by cluster, namespace, node, deployment</li>
                <li>Optional in-cluster gateway so the token lives in one Secret</li>
              </ul>
            </div>
            <Terminal
              title="bash · cluster instrumentation"
              lines={[
                {
                  ln: "1",
                  type: "tx",
                  content: (
                    <>
                      <span className="cmd">$</span> kubectl create secret
                      generic traceway-token -n traceway \
                    </>
                  ),
                },
                {
                  ln: "2",
                  type: "tx",
                  content: "    --from-literal=token=$TRACEWAY_PROJECT_TOKEN",
                },
                {
                  ln: "3",
                  type: "tx",
                  content: (
                    <>
                      <span className="cmd">$</span> kubectl apply -f
                      traceway-kubernetes.yaml
                    </>
                  ),
                },
                {
                  ln: "4",
                  type: "mute",
                  content: "daemonset.apps/traceway-node-agent created",
                },
                {
                  ln: "5",
                  type: "mute",
                  content: "deployment.apps/traceway-cluster-agent created",
                },
                {
                  ln: "6",
                  type: "ok",
                  content: "✓ 12 nodes reporting under production-eu",
                },
              ]}
            />
          </div>
        </section>

        <section className="wrap">
          <FeatureRow
            eyebrow="Across every project"
            title={
              <>
                Uptime and incidents, <em>without the project hop</em>
              </>
            }
            description="Every monitor in the organization in one sortable list with its project, status, 30-day uptime, and average latency, over the organization's whole incident history. The Issues tab does the same for exceptions: everything that fired in the last 24 hours, newest first, labelled with the project that owns it. Triage first, drill in second."
            bullets={[
              "Monitors, uptime, latency, and last incident across all projects",
              "90 days of incident history, ongoing ones marked",
              "Recently active issues from every project in one feed",
              "Uptime excludes missed probes, so downtime never reads as 100%",
            ]}
            image={{
              src: "/images/organization-monitors.png",
              alt: "The organization Monitors tab: every monitor across projects with status, uptime, and average latency, plus recent incidents",
            }}
          />
        </section>

        <section className="wrap">
          <FeatureRow
            reverse
            eyebrow="Organizations and projects"
            title={
              <>
                One switcher for <em>everything you can see</em>
              </>
            }
            description="Projects are the isolation boundary for telemetry. Organizations own everything above them: members and roles, dashboards, teams, on-call rotations, escalation policies, and status pages. Belong to several and each keeps its own overview, which is what makes an organization the right shape for an agency's clients or a company's independent units."
            bullets={[
              "Organizations as headings, their projects nested underneath",
              "Every organization member can read the overview, read-only included",
              "Per-project role overrides shown as your effective access",
              "Sign in and land on the overview when you have more than one project",
            ]}
            image={{
              src: "/images/organization-switcher.png",
              alt: "The Traceway header switcher, listing organizations with their projects nested underneath",
            }}
          />
        </section>

        <section className="wrap">
          <FeatureRow
            eyebrow="From fleet to one box"
            title={
              <>
                Click a row, <em>land on the answer</em>
              </>
            }
            description="Selecting an instance opens its project's dashboards already filtered to that one machine over the last 30 minutes, on the server dashboard if the project has one installed. The scope is a URL parameter, so the filtered view is shareable and survives every time-range change you make while digging."
            bullets={[
              "Instance picker lists what actually reported in the range",
              "Applies on top of each widget's own filters, saves nothing",
              "Open on-call pages link straight to the page that fired",
              "Issue rows link to the exception in its own project",
            ]}
            image={{
              src: "/images/organization-issues.png",
              alt: "The organization Issues tab: recently active issues from every project with the project each belongs to",
            }}
          />
        </section>
      </div>

      <FinalCTA
        title={
          <>
            Stop opening <em>ten dashboards</em>
          </>
        }
        description="Fleet health, cross-project triage, Kubernetes, and on-call in the same tool that already holds your traces, logs, and errors. Open source if you self-host."
        primary={{
          label: "Read the Kubernetes docs",
          href: "https://docs.tracewayapp.com/learn/kubernetes",
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
            title="Questions about fleets and clusters"
          />
          <div className="mt-4">
            <FaqList
              items={[
                {
                  q: "What counts as an instance?",
                  a: (
                    <>
                      <p>
                        Anything reporting server metrics under its own{" "}
                        <code>service.name</code>, which Traceway stores as the{" "}
                        <code>server_name</code> tag. A VM with the OTel Agent, a
                        bare-metal box, a Kubernetes node. Instances are
                        discovered from the metrics themselves, so there is
                        nothing to register and nothing to delete when a machine
                        goes away.
                      </p>
                    </>
                  ),
                },
                {
                  q: "Are pods instances?",
                  a: "No, nodes are. A pod moves, restarts, and gets a new name on every rollout, so a per-pod instance list would be noise. The node agent names each node, and pod identity lives on the metrics as k8s.pod.name, k8s.namespace.name, and k8s.deployment.name, which you group dashboards by. Applications keep their own service.name, so an API stays one service in Endpoints and Issues no matter which node it lands on.",
                },
                {
                  q: "Can one organization hold several clusters?",
                  a: "Yes, and several clusters can even share one project. Each cluster reports its own k8s.cluster.name, and the overview groups instances by it. Machines with no cluster attribute collect under \"Not in a Kubernetes cluster\", so a migration in progress still shows as one fleet rather than two tools.",
                },
                {
                  q: "Do I need the OTel Agent as well as the Kubernetes collectors?",
                  a: "Not on the same machine. The agent is a host service for VMs and bare metal; the DaemonSet covers nodes inside a cluster and reports the same hostmetrics, plus pod metrics, container logs, and cluster state. Most fleets run both, just not on the same box.",
                },
                {
                  q: "Does the overview leak data across teams?",
                  a: "No. It shows the organization's projects, which every member of that organization can already open, and it respects per-project role overrides: the Projects tab lists your effective access on each one. Read-only members can open it; it grants nothing they did not already have. Nothing is shared between separate organizations.",
                },
                {
                  q: "How current is it?",
                  a: "The instance table covers the last 30 minutes and the page refreshes itself every minute, with a manual refresh button. An instance is stale after three minutes of silence. A refresh that fails leaves the last good snapshot on screen with a warning instead of blanking the page, and one unreadable project never takes down the rest.",
                },
                {
                  q: "Is this a paid feature?",
                  a: "No. The organization overview and the Kubernetes manifests are part of self-hosted Traceway, which is fully open source, and part of every Traceway Cloud plan. Your plan sizes what you can send, not which views you get.",
                },
              ]}
            />
          </div>
        </div>
      </section>
    </main>
  );
}
