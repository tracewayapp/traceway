import Image from "next/image";
import Link from "next/link";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from "@/components/ui/accordion";
import {
  Github,
  ArrowRight,
  Bug,
  Video,
  Activity,
  Lock,
  Terminal,
  GitCompare,
} from "lucide-react";
import { CodeTabs } from "@/components/code-tabs";
import { ImpactScoreVisual } from "@/components/impact-score-visual";
import { CostComparison } from "@/components/cost-comparison";
import { DockerCommand } from "@/components/docker-command";

const frameworks = [
  { name: "Gin", src: "/images/frameworks/gin.png" },
  { name: "Chi", src: "/images/frameworks/chi.png" },
  { name: "Stdlib", src: "/images/frameworks/stdlib.png" },
  { name: "FastHTTP", src: "/images/frameworks/fasthttp.png" },
  { name: "Express", src: "/images/frameworks/express.png" },
  { name: "NestJS", src: "/images/frameworks/nestjs.png" },
  { name: "Next.js", src: "/images/frameworks/nextjs.png" },
  { name: "Node.js", src: "/images/frameworks/node.png" },
  { name: "Svelte", src: "/images/frameworks/svelte.png" },
  { name: "Remix", src: "/images/frameworks/remix.png" },
  { name: "OpenTelemetry", src: "/images/frameworks/otel.png" },
];

export default function Home() {
  return (
    <main className="min-h-screen bg-white text-zinc-950 font-sans selection:bg-zinc-100 selection:text-zinc-900">
      {/* Section 1: Hero */}
      <section className="relative pt-16 pb-20 overflow-hidden">
        <div className="absolute inset-0 -z-10 h-full w-full bg-white bg-[radial-gradient(#e5e7eb_1px,transparent_1px)] [background-size:16px_16px] [mask-image:radial-gradient(ellipse_50%_50%_at_50%_50%,#000_70%,transparent_100%)]"></div>

        <div className="container mx-auto px-4 relative z-10 text-center">
          <Link href="https://github.com/tracewayapp/traceway" target="_blank">
            <span className="inline-flex items-center mb-4 bg-green-50 text-green-700 hover:bg-green-100 px-2.5 py-0.5 border border-green-200 text-xs font-medium rounded-full cursor-pointer transition-colors">
              100% Open Source
            </span>
          </Link>
          <h1 className="text-4xl md:text-6xl font-bold tracking-tight mb-6 text-zinc-900">
            Know what to fix first
          </h1>
          <p className="text-zinc-600 text-lg md:text-xl max-w-2xl mx-auto mb-10 leading-relaxed font-medium">
            Traceway scores every endpoint and exception by real user impact,
            then ranks them. You open the dashboard and immediately know what
            needs attention. No triage meetings. No alert fatigue.
          </p>
          <div className="flex flex-col sm:flex-row items-center justify-center gap-3">
            <Link href="https://docs.tracewayapp.com">
              <Button
                size="lg"
                className="h-10 px-6 text-sm bg-zinc-900 text-white hover:bg-zinc-800 shadow-lg shadow-zinc-900/20"
              >
                Get Started <ArrowRight className="ml-2 h-4 w-4" />
              </Button>
            </Link>
            <Link
              href="https://github.com/tracewayapp/traceway"
              target="_blank"
            >
              <Button
                variant="outline"
                size="lg"
                className="h-10 px-6 text-sm border-zinc-200 bg-white hover:bg-zinc-50 text-zinc-900 shadow-sm"
              >
                <Github className="mr-2 h-4 w-4" /> View on GitHub
              </Button>
            </Link>
          </div>
          <div className="mt-4">
            <Link
              href="https://cloud.tracewayapp.com/login?email=demo@tracewayapp.com&password=demoaccount!"
              className="text-sm text-zinc-500 hover:text-zinc-700 transition-colors"
            >
              or try the live demo <ArrowRight className="inline h-3 w-3" />
            </Link>
          </div>
        </div>
      </section>

      {/* Section 2: Framework Logos */}
      <section className="pt-0 pb-10 bg-white border-b border-zinc-100">
        <div className="container mx-auto px-4 max-w-4xl">
          <p className="text-center text-xs font-semibold text-zinc-400 uppercase tracking-wider mb-6">
            Works with OpenTelemetry, Go, JavaScript
          </p>
          <div className="flex flex-wrap items-center justify-center gap-8 md:gap-10">
            {frameworks.map((fw) => (
              <Image
                key={fw.name}
                src={fw.src}
                alt={fw.name}
                width={40}
                height={40}
                className="h-8 w-auto opacity-80 hover:opacity-100 transition-all duration-200"
              />
            ))}
          </div>
        </div>
      </section>

      {/* Section 3: Impact Score */}
      <section className="py-20 bg-zinc-50/50 border-b border-zinc-100">
        <div className="container mx-auto px-4 max-w-5xl">
          <div className="text-center mb-12">
            <h2 className="text-3xl md:text-4xl font-bold mb-4 text-zinc-900 tracking-tight">
              One score. Five signals. Zero guesswork.
            </h2>
            <p className="text-zinc-600 text-lg max-w-2xl mx-auto">
              The Impact Score combines five service-level indicators into one
              automatic priority for every endpoint. It takes the max across all
              five - if any single signal is bad, the endpoint surfaces
              immediately.
            </p>
          </div>
          <div className="max-w-4xl mx-auto overflow-x-auto">
            <ImpactScoreVisual />
          </div>
        </div>
      </section>

      {/* Section 4: Feature Showcase */}
      <section className="py-24 bg-white border-b border-zinc-100">
        <div className="container mx-auto px-4 max-w-5xl space-y-32">
          {/* 4a: Exception Tracking */}
          <div className="flex flex-col md:flex-row items-center gap-12 lg:gap-20">
            <div className="flex-1 space-y-6">
              <div className="w-12 h-12 bg-red-50 rounded-2xl flex items-center justify-center">
                <Bug className="w-6 h-6 text-red-600" />
              </div>
              <h3 className="text-2xl md:text-3xl font-bold text-zinc-900 tracking-tight">
                Every exception, grouped and ranked
              </h3>
              <p className="text-zinc-600 text-lg leading-relaxed">
                Full stack traces, 10-step normalization, SHA-256 grouping.
                Thousands of duplicates collapse into one ranked issue so you
                fix what matters first.
              </p>
              <ul className="space-y-3 pt-2">
                <li className="flex items-center gap-3 text-zinc-700">
                  <div className="w-1.5 h-1.5 rounded-full bg-red-500"></div>
                  Full stack trace capture
                </li>
                <li className="flex items-center gap-3 text-zinc-700">
                  <div className="w-1.5 h-1.5 rounded-full bg-red-500"></div>
                  Intelligent error grouping
                </li>
                <li className="flex items-center gap-3 text-zinc-700">
                  <div className="w-1.5 h-1.5 rounded-full bg-red-500"></div>
                  User impact analysis
                </li>
              </ul>
            </div>
            <div className="flex-1 w-full relative">
              <div className="absolute inset-0 bg-gradient-to-tr from-red-100/50 to-transparent rounded-3xl transform rotate-3 scale-105 -z-10"></div>
              <div className="relative rounded-xl overflow-hidden border border-zinc-200 bg-white">
                <Image
                  src="/images/screenshot-2.png"
                  alt="Exception Tracking Interface"
                  width={800}
                  height={600}
                  className="w-full h-auto"
                />
              </div>
            </div>
          </div>

          {/* 4b: Session Replay */}
          <div className="flex flex-col md:flex-row-reverse items-center gap-12 lg:gap-20">
            <div className="flex-1 space-y-6">
              <div className="w-12 h-12 bg-purple-50 rounded-2xl flex items-center justify-center">
                <Video className="w-6 h-6 text-purple-600" />
              </div>
              <h3 className="text-2xl md:text-3xl font-bold text-zinc-900 tracking-tight">
                See exactly what the user did
              </h3>
              <p className="text-zinc-600 text-lg leading-relaxed">
                Traceway captures ~10 seconds of user activity before every
                error. Clicks, scrolls, and form interactions are attached to
                exceptions automatically - no manual reproduction needed.
              </p>
              <ul className="space-y-3 pt-2">
                <li className="flex items-center gap-3 text-zinc-700">
                  <div className="w-1.5 h-1.5 rounded-full bg-purple-500"></div>
                  Pre-error activity capture
                </li>
                <li className="flex items-center gap-3 text-zinc-700">
                  <div className="w-1.5 h-1.5 rounded-full bg-purple-500"></div>
                  Automatic attachment to exceptions
                </li>
                <li className="flex items-center gap-3 text-zinc-700">
                  <div className="w-1.5 h-1.5 rounded-full bg-purple-500"></div>
                  Clicks, scrolls, and form interactions
                </li>
              </ul>
            </div>
            <div className="flex-1 w-full relative">
              <div className="absolute inset-0 bg-gradient-to-tl from-purple-100/50 to-transparent rounded-3xl transform -rotate-3 scale-105 -z-10"></div>
              <div className="relative rounded-xl overflow-hidden border border-zinc-200 bg-white">
                <Image
                  src="/images/session-replay.png"
                  alt="Session Replay Interface"
                  width={800}
                  height={600}
                  className="w-full h-auto"
                />
              </div>
            </div>
          </div>

          {/* 4c: Endpoint Introspection */}
          <div className="flex flex-col md:flex-row items-center gap-12 lg:gap-20">
            <div className="flex-1 space-y-6">
              <div className="w-12 h-12 bg-blue-50 rounded-2xl flex items-center justify-center">
                <Activity className="w-6 h-6 text-blue-600" />
              </div>
              <h3 className="text-2xl md:text-3xl font-bold text-zinc-900 tracking-tight">
                Drill into any request
              </h3>
              <p className="text-zinc-600 text-lg leading-relaxed">
                Request/response details, waterfall traces, and custom context
                tags. Understand the exact state of your application for every
                single trace.
              </p>
              <ul className="space-y-3 pt-2">
                <li className="flex items-center gap-3 text-zinc-700">
                  <div className="w-1.5 h-1.5 rounded-full bg-blue-500"></div>
                  Detailed request/response data
                </li>
                <li className="flex items-center gap-3 text-zinc-700">
                  <div className="w-1.5 h-1.5 rounded-full bg-blue-500"></div>
                  Waterfall trace view
                </li>
                <li className="flex items-center gap-3 text-zinc-700">
                  <div className="w-1.5 h-1.5 rounded-full bg-blue-500"></div>
                  Custom context & tagging
                </li>
              </ul>
            </div>
            <div className="flex-1 w-full relative">
              <div className="absolute inset-0 bg-gradient-to-tr from-blue-100/50 to-transparent rounded-3xl transform rotate-3 scale-105 -z-10"></div>
              <div className="relative rounded-xl overflow-hidden border border-zinc-200 bg-white">
                <Image
                  src="/images/screenshot-1.png"
                  alt="Endpoint Introspection"
                  width={800}
                  height={600}
                  className="w-full h-auto"
                />
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* Section 5: Open Source & Self-Hosting */}
      <section className="py-20 bg-white border-b border-zinc-100">
        <div className="container mx-auto px-4 max-w-5xl">
          <div className="text-center mb-12">
            <h2 className="text-3xl md:text-4xl font-bold mb-4 text-zinc-900 tracking-tight">
              100% open source. No asterisks.
            </h2>
            <p className="text-zinc-600 text-lg max-w-2xl mx-auto">
              Not BSL. Not source-available. Not &ldquo;open core.&rdquo; Every
              feature works identically self-hosted or on cloud. Deploy with a
              single Docker command.
            </p>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
            <Card className="bg-white border-zinc-200 transition-all duration-300">
              <CardHeader className="p-6 pt-0">
                <div className="w-10 h-10 bg-green-50 rounded-lg flex items-center justify-center mb-3">
                  <Lock className="w-5 h-5 text-green-600" />
                </div>
                <CardTitle className="text-lg">Truly Open Source</CardTitle>
                <CardDescription className="text-zinc-500 text-sm mt-1.5">
                  Unlike Sentry&apos;s BSL license, Traceway is fully open
                  source. Fork it, modify it, run it however you want.
                </CardDescription>
              </CardHeader>
            </Card>

            <Card className="bg-white border-zinc-200 transition-all duration-300">
              <CardHeader className="p-6 pt-0">
                <div className="w-10 h-10 bg-blue-50 rounded-lg flex items-center justify-center mb-3">
                  <Terminal className="w-5 h-5 text-blue-600" />
                </div>
                <CardTitle className="text-lg">One-Command Deploy</CardTitle>
                <CardDescription className="text-zinc-500 text-sm mt-1.5">
                  Get up and running with a single command. No complex
                  configuration or infrastructure setup required.
                </CardDescription>
              </CardHeader>
            </Card>

            <Card className="bg-white border-zinc-200 transition-all duration-300">
              <CardHeader className="p-6 pt-0">
                <div className="w-10 h-10 bg-orange-50 rounded-lg flex items-center justify-center mb-3">
                  <GitCompare className="w-5 h-5 text-orange-600" />
                </div>
                <CardTitle className="text-lg">Same Code Everywhere</CardTitle>
                <CardDescription className="text-zinc-500 text-sm mt-1.5">
                  No feature gating between self-hosted and cloud. The exact
                  same codebase powers both.
                </CardDescription>
              </CardHeader>
            </Card>
          </div>

          <div className="flex justify-center mt-10">
            <DockerCommand />
          </div>
        </div>
      </section>

      {/* Section 7: OpenTelemetry Support */}
      <section className="py-16 bg-zinc-50/50 border-b border-zinc-100">
        <div className="container mx-auto px-4 max-w-3xl text-center">
          <Image
            src="/images/frameworks/otel.png"
            alt="OpenTelemetry"
            width={48}
            height={48}
            className="mx-auto mb-6 h-12 w-auto"
          />
          <h2 className="text-3xl md:text-4xl font-bold mb-4 text-zinc-900 tracking-tight">
            Send from any OpenTelemetry source
          </h2>
          <p className="text-zinc-600 text-lg max-w-xl mx-auto">
            Already instrumented with OTel? Point your OTLP exporter at
            Traceway. No proprietary SDK lock-in.
          </p>
        </div>
      </section>

      {/* Section 8: Quick Integration */}
      <section className="py-16 bg-white border-b border-zinc-100">
        <div className="container mx-auto px-4 max-w-5xl">
          <div className="flex flex-col md:flex-row items-center justify-between gap-12">
            <div className="flex-1 space-y-4">
              <h2 className="text-2xl font-bold tracking-tight text-zinc-900">
                No OpenTelemetry? No Problem!
              </h2>
              <h3 className="text-xl italic text-zinc-700">
                Two lines of code. That&apos;s it.
              </h3>
              <p className="text-zinc-600 text-base leading-relaxed max-w-md">
                Add the middleware to your router and start collecting
                actionable telemetry instantly. No complex configuration
                required.
              </p>
            </div>
            <div className="flex-1 w-full max-w-lg">
              <CodeTabs />
            </div>
          </div>
        </div>
      </section>

      {/* Section 9: Demo CTA */}
      <section className="py-8 bg-zinc-50/50 border-b border-zinc-100">
        <div className="container mx-auto px-4 max-w-3xl text-center">
          <h2 className="text-3xl md:text-4xl font-bold mb-4 text-zinc-900 tracking-tight">
            See it in action
          </h2>
          <p className="text-zinc-600 text-lg mb-4">
            Explore a live demo with sample data, no signup required.
          </p>
          <Link href="https://cloud.tracewayapp.com/login?email=demo@tracewayapp.com&password=demoaccount!">
            <Button
              size="lg"
              className="h-10 px-4 text-sm bg-zinc-900 text-white hover:bg-zinc-800 shadow-lg shadow-zinc-900/20"
            >
              Try Live Demo <ArrowRight className="ml-2 h-4 w-4" />
            </Button>
          </Link>
        </div>
      </section>

      {/* Section 10: Pricing & Cost Advantage */}
      <section className="py-20 bg-zinc-50/50 border-b border-zinc-100">
        <div className="container mx-auto px-4 max-w-5xl">
          <div className="text-center mb-12">
            <h2 className="text-3xl md:text-4xl font-bold mb-4 text-zinc-900 tracking-tight">
              Designed for efficiency. Built to lower your cloud bill.
            </h2>
            <p className="text-zinc-600 text-lg max-w-2xl mx-auto">
              Traceway runs lean. ClickHouse columnar storage compresses 1
              million daily events into ~2-3 GB per month. Postgres is used for
              efficient user and organization storage.
            </p>
          </div>
          <CostComparison />
          <div className="flex flex-col sm:flex-row items-center justify-center gap-3 mt-10">
            <Link href="/cloud">
              <Button
                size="lg"
                className="h-10 px-6 text-sm bg-[#4ba3f7] text-white hover:bg-[#3b93e7] shadow-lg shadow-blue-900/10"
              >
                See Cloud pricing <ArrowRight className="ml-2 h-4 w-4" />
              </Button>
            </Link>
            <Link href="https://docs.tracewayapp.com">
              <Button
                variant="outline"
                size="lg"
                className="h-10 px-6 text-sm border-zinc-200 bg-white hover:bg-zinc-50 text-zinc-900 shadow-sm"
              >
                Self-host for free <ArrowRight className="ml-2 h-4 w-4" />
              </Button>
            </Link>
          </div>
        </div>
      </section>

      {/* Section 11: FAQ */}
      <section className="py-24 bg-zinc-50 border-t border-zinc-100">
        <div className="container mx-auto px-4 max-w-3xl">
          <div className="text-center mb-12">
            <h2 className="text-3xl font-bold mb-4 text-zinc-900 tracking-tight">
              Frequently Asked Questions
            </h2>
            <p className="text-zinc-600 text-lg">
              Everything you need to know about Traceway.
            </p>
          </div>

          <Accordion type="single" collapsible className="w-full">
            <AccordionItem value="item-1" className="border-b-zinc-200">
              <AccordionTrigger className="text-zinc-900 hover:text-zinc-700 hover:no-underline text-left">
                What is the Impact Score?
              </AccordionTrigger>
              <AccordionContent className="text-zinc-600 leading-relaxed">
                The Impact Score is Traceway&apos;s automatic prioritization
                system. It combines five service-level indicators - inverted
                apdex variant, error rate floor, P99 floor, client error floor,
                and volume error floor - into a single score for every endpoint.
                It takes the max across all five, so if any single signal is
                degraded, that endpoint surfaces immediately. You open the
                dashboard and instantly know what needs attention.
              </AccordionContent>
            </AccordionItem>
            <AccordionItem value="item-2" className="border-b-zinc-200">
              <AccordionTrigger className="text-zinc-900 hover:text-zinc-700 hover:no-underline text-left">
                How does Traceway compare to Sentry?
              </AccordionTrigger>
              <AccordionContent className="text-zinc-600 leading-relaxed">
                Sentry is a great tool, but it requires manual triage and uses a
                BSL license. Traceway automatically ranks issues by real user
                impact so you always know what to fix first. It&apos;s 100% open
                source (not BSL, not source-available), runs on your
                infrastructure with fixed costs, and combines exception
                tracking, performance monitoring, and session replay in one
                tool.
              </AccordionContent>
            </AccordionItem>
            <AccordionItem value="item-3" className="border-b-zinc-200">
              <AccordionTrigger className="text-zinc-900 hover:text-zinc-700 hover:no-underline text-left">
                How does Traceway compare to Datadog/New Relic?
              </AccordionTrigger>
              <AccordionContent className="text-zinc-600 leading-relaxed">
                Datadog and New Relic are powerful but expensive at scale, often
                charging per event or per host. Traceway runs on your
                infrastructure with fixed costs. ClickHouse columnar storage
                compresses data dramatically, so 1 million daily events use only
                ~2-3 GB per month. You get endpoint analytics, exception
                tracking, and session replay without metered billing.
              </AccordionContent>
            </AccordionItem>
            <AccordionItem value="item-4" className="border-b-zinc-200">
              <AccordionTrigger className="text-zinc-900 hover:text-zinc-700 hover:no-underline text-left">
                Is Traceway really free to self-host?
              </AccordionTrigger>
              <AccordionContent className="text-zinc-600 leading-relaxed">
                Yes. Traceway is 100% open source with no feature gating. Every
                feature available on Traceway Cloud works identically when
                self-hosted. Deploy with{" "}
                <code className="bg-zinc-100 px-1 py-0.5 rounded text-sm">
                  docker compose up -d
                </code>{" "}
                and you&apos;re running.
              </AccordionContent>
            </AccordionItem>
            <AccordionItem value="item-5" className="border-b-zinc-200">
              <AccordionTrigger className="text-zinc-900 hover:text-zinc-700 hover:no-underline text-left">
                How does error grouping work?
              </AccordionTrigger>
              <AccordionContent className="text-zinc-600 leading-relaxed">
                Traceway applies a 10-step normalization pipeline to every stack
                trace: extracting the error type, removing absolute file paths,
                replacing hex addresses, UUIDs, IPs, timestamps, and numeric IDs
                with placeholders, normalizing whitespace, and stripping ANSI
                codes. The result is hashed with SHA-256 so identical logical
                errors always group together, even if runtime values differ.
              </AccordionContent>
            </AccordionItem>
            <AccordionItem value="item-6" className="border-b-zinc-200">
              <AccordionTrigger className="text-zinc-900 hover:text-zinc-700 hover:no-underline text-left">
                What is session replay?
              </AccordionTrigger>
              <AccordionContent className="text-zinc-600 leading-relaxed">
                Session replay records DOM changes in the browser. When an error
                occurs, Traceway captures approximately 10 seconds of user
                activity leading up to the exception - clicks, scrolls, form
                interactions, and page navigations. The replay is attached to
                the exception automatically, so you can see exactly what the
                user did without asking them to reproduce the issue.
              </AccordionContent>
            </AccordionItem>
            <AccordionItem value="item-7" className="border-b-zinc-200">
              <AccordionTrigger className="text-zinc-900 hover:text-zinc-700 hover:no-underline text-left">
                Does Traceway support OpenTelemetry?
              </AccordionTrigger>
              <AccordionContent className="text-zinc-600 leading-relaxed">
                Yes. If your services are already instrumented with
                OpenTelemetry, you can point your OTLP exporter at Traceway and
                start receiving data immediately. There&apos;s no proprietary
                SDK lock-in - use Traceway&apos;s lightweight middleware or any
                OTel-compatible instrumentation.
              </AccordionContent>
            </AccordionItem>
          </Accordion>
        </div>
      </section>
    </main>
  );
}
