import Image from "next/image";
import Link from "next/link";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Accordion, AccordionContent, AccordionItem, AccordionTrigger } from "@/components/ui/accordion";
import { Github, ArrowRight, Activity, TrendingUp, AlertCircle, Bug, ChartGantt } from "lucide-react";

export default function Home() {
  return (
    <div className="min-h-screen bg-white text-zinc-950 font-sans selection:bg-zinc-100 selection:text-zinc-900">
      {/* Navigation */}
      <nav className="border-b border-zinc-100 bg-white/80 backdrop-blur-md sticky top-0 z-50">
        <div className="container mx-auto px-4 h-14 flex items-center justify-between">
          <div className="flex items-center gap-2">
            <Image
              src="/images/logo.png"
              alt="Traceway Logo"
              width={100}
              height={100}
              className="w-auto h-8"
            />
          </div>
          <div className="flex items-center gap-4">
            <Link href="https://docs.tracewayapp.com" className="text-sm font-medium text-zinc-600 hover:text-zinc-900 transition-colors">
              Documentation
            </Link>
            <Link href="https://github.com/tracewayapp/traceway" target="_blank" rel="noopener noreferrer">
              <Button variant="ghost" size="icon" className="h-8 w-8 text-zinc-600 hover:text-zinc-900 hover:bg-zinc-100">
                <Github className="h-4 w-4" />
                <span className="sr-only">GitHub</span>
              </Button>
            </Link>
          </div>
        </div>
      </nav>

      {/* Hero Section */}
      <section className="relative pt-16 pb-20 overflow-hidden">
        {/* Dot Pattern Background */}
        <div className="absolute inset-0 -z-10 h-full w-full bg-white bg-[radial-gradient(#e5e7eb_1px,transparent_1px)] [background-size:16px_16px] [mask-image:radial-gradient(ellipse_50%_50%_at_50%_50%,#000_70%,transparent_100%)]"></div>

        <div className="container mx-auto px-4 relative z-10 text-center">
          <Badge variant="secondary" className="mb-4 bg-zinc-100 text-zinc-600 hover:bg-zinc-200 px-2.5 py-0.5 border border-zinc-200 text-xs font-normal">
            Star us on GitHub
          </Badge>
          <h1 className="text-4xl md:text-6xl font-bold tracking-tight mb-6 text-zinc-900">
            Telemetry & Issue Tracking <br /> for <span className="text-transparent bg-clip-text bg-gradient-to-r from-blue-600 to-indigo-600">Golang</span>
          </h1>
          <p className="text-zinc-600 text-lg md:text-xl max-w-xl mx-auto mb-10 leading-relaxed font-medium">
            Ship fast and debug less with Traceway. Simple integration, performance insights, and regression tracking.
          </p>
          <div className="flex flex-col sm:flex-row items-center justify-center gap-3">
            <Link href="https://docs.tracewayapp.com">
              <Button size="lg" className="h-10 px-6 text-sm bg-zinc-900 text-white hover:bg-zinc-800 shadow-lg shadow-zinc-900/20">
                Get Started <ArrowRight className="ml-2 h-4 w-4" />
              </Button>
            </Link>
            <Link href="https://github.com/tracewayapp/traceway" target="_blank">
              <Button variant="outline" size="lg" className="h-10 px-6 text-sm border-zinc-200 bg-white hover:bg-zinc-50 text-zinc-900 shadow-sm">
                <Github className="mr-2 h-4 w-4" /> View on GitHub
              </Button>
            </Link>
          </div>
        </div>
      </section>

      {/* Code Snippet Section */}
      <section className="py-16 bg-zinc-50/50 border-y border-zinc-100">
        <div className="container mx-auto px-4 max-w-5xl">
          <div className="flex flex-col md:flex-row items-center justify-between gap-12">
            <div className="flex-1 space-y-4">
              <h2 className="text-2xl font-bold tracking-tight text-zinc-900">2-Minute Integration</h2>
              <p className="text-zinc-600 text-base leading-relaxed max-w-md">
                Add the middleware to your Gin router and start collecting actionable telemetry instantly.
                No complex configuration required.
              </p>
              <ul className="space-y-3 pt-2">
                <li className="flex items-center gap-2.5 text-zinc-700 font-medium text-sm">
                  <div className="h-6 w-6 rounded-full bg-blue-100 flex items-center justify-center text-blue-600 text-xs font-bold">1</div>
                  Install the package
                </li>
                <li className="flex items-center gap-2.5 text-zinc-700 font-medium text-sm">
                  <div className="h-6 w-6 rounded-full bg-blue-100 flex items-center justify-center text-blue-600 text-xs font-bold">2</div>
                  Add middleware
                </li>
                <li className="flex items-center gap-2.5 text-zinc-700 font-medium text-sm">
                  <div className="h-6 w-6 rounded-full bg-blue-100 flex items-center justify-center text-blue-600 text-xs font-bold">3</div>
                  View insights
                </li>
              </ul>
            </div>
            <div className="flex-1 w-full max-w-lg">
              <div className="rounded-lg overflow-hidden bg-white border border-zinc-200 shadow-xl shadow-zinc-200/50">
                <div className="flex items-center justify-between px-3 py-2 bg-zinc-50 border-b border-zinc-100">
                  <div className="flex items-center gap-1.5">
                    <div className="w-2.5 h-2.5 rounded-full bg-[#fe5f57]"></div>
                    <div className="w-2.5 h-2.5 rounded-full bg-[#fdbc2e]"></div>
                    <div className="w-2.5 h-2.5 rounded-full bg-[#28c841]"></div>
                  </div>
                  <span className="text-[10px] text-zinc-500 font-mono font-medium">main.go</span>
                </div>
                <div className="p-0 overflow-x-auto bg-white">
                  <pre className="p-4 text-xs font-mono leading-relaxed text-zinc-800">
                    <code className="block">
                      <span className="text-purple-600 font-semibold">func</span> <span className="text-blue-600 font-semibold">main</span>() {"{"}{"\n"}
                      {"  "}r := gin.<span className="text-blue-600">Default</span>(){"\n"}
                      {"  "}r.<span className="text-blue-600">Use</span>(traceway_gin.<span className="text-blue-600">New</span>(<span className="text-green-600">"{`{TOKEN}`}@https://{`{SERVER_URL}`}/api/report"</span>)){"\n"}
                      {"\n"}
                      {"  "}r.<span className="text-blue-600">GET</span>(<span className="text-green-600">"/test"</span>, <span className="text-purple-600 font-semibold">func</span>(ctx *gin.Context) {"{"}{"\n"}
                      {"    "}ctx.<span className="text-blue-600">AbortWithError</span>(<span className="text-orange-600">500</span>, fmt.<span className="text-blue-600">Errorf</span>(<span className="text-green-600">"Worked!"</span>)){"\n"}
                      {"  "}{"}"}){"\n"}
                      {"  "}r.<span className="text-blue-600">Run</span>(<span className="text-green-600">":8080"</span>){"\n"}
                      {"}"}
                    </code>
                  </pre>
                </div>
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* Features Section */}
      <section className="py-20 bg-white">
        <div className="container mx-auto px-4 max-w-5xl">
          <div className="text-center mb-12">
            <h2 className="text-3xl md:text-4xl font-bold mb-4 text-zinc-900 tracking-tight">Built for Reliability</h2>
            <p className="text-zinc-600 text-lg max-w-xl mx-auto">
              Traceway treats every error as a signal. We help you cut through the noise and find exactly what broke.
            </p>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
            <Card className="bg-white border-zinc-200 shadow-sm hover:shadow-md transition-all duration-300">
              <CardHeader className="p-6 pt-0">
                <div className="w-10 h-10 bg-blue-50 rounded-lg flex items-center justify-center mb-3">
                  <TrendingUp className="w-5 h-5 text-blue-600" />
                </div>
                <CardTitle className="text-lg">Actionable Insights</CardTitle>
                <CardDescription className="text-zinc-500 text-sm mt-1.5">
                  We verify and rank issues so you know what to prioritize. No more digging through log files.
                </CardDescription>
              </CardHeader>
            </Card>

            <Card className="bg-white border-zinc-200 shadow-sm hover:shadow-md transition-all duration-300">
              <CardHeader className="p-6 pt-0">
                <div className="w-10 h-10 bg-green-50 rounded-lg flex items-center justify-center mb-3">
                  <Activity className="w-5 h-5 text-green-600" />
                </div>
                <CardTitle className="text-lg">Regression Tracking</CardTitle>
                <CardDescription className="text-zinc-500 text-sm mt-1.5">
                  Automatically track when issues reappear. Keep your production environment clean and stable.
                </CardDescription>
              </CardHeader>
            </Card>

            <Card className="bg-white border-zinc-200 shadow-sm hover:shadow-md transition-all duration-300">
              <CardHeader className="p-6 pt-0">
                <div className="w-10 h-10 bg-orange-50 rounded-lg flex items-center justify-center mb-3">
                  <AlertCircle className="w-5 h-5 text-orange-600" />
                </div>
                <CardTitle className="text-lg">Error Grouping</CardTitle>
                <CardDescription className="text-zinc-500 text-sm mt-1.5">
                  Intelligent grouping of similar errors. See the impact of a bug at a glance.
                </CardDescription>
              </CardHeader>
            </Card>
          </div>
        </div>
      </section>

      {/* Feature Sections */}
      <section className="py-24 bg-white border-y border-zinc-100">
        <div className="container mx-auto px-4 max-w-5xl space-y-32">
          {/* Feature 1: Exception Tracking */}
          <div className="flex flex-col md:flex-row items-center gap-12 lg:gap-20">
            <div className="flex-1 space-y-6">
              <div className="w-12 h-12 bg-red-50 rounded-2xl flex items-center justify-center">
                <Bug className="w-6 h-6 text-red-600" />
              </div>
              <h3 className="text-2xl md:text-3xl font-bold text-zinc-900 tracking-tight">Granular exception tracking</h3>
              <p className="text-zinc-600 text-lg leading-relaxed">
                Traceway captures every exception with full stack traces and context.
                We group similar errors together, so you can see exactly how many times an issue occurred,
                when it started, and which users are affected.
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
              <div className="relative rounded-xl overflow-hidden border border-zinc-200 shadow-2xl shadow-zinc-200/50 bg-white">
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

          {/* Feature 2: Introspection (Endpoint Details) */}
          <div className="flex flex-col md:flex-row-reverse items-center gap-12 lg:gap-20">
            <div className="flex-1 space-y-6">
              <div className="w-12 h-12 bg-blue-50 rounded-2xl flex items-center justify-center">
                <Activity className="w-6 h-6 text-blue-600" />
              </div>
              <h3 className="text-2xl md:text-3xl font-bold text-zinc-900 tracking-tight">Deep endpoint introspection</h3>
              <p className="text-zinc-600 text-lg leading-relaxed">
                Go beyond simple metrics. Inspect individual requests to understand the exact state of your application.
                View headers, payload sizes, and custom context variables for every single trace.
              </p>
              <ul className="space-y-3 pt-2">
                <li className="flex items-center gap-3 text-zinc-700">
                  <div className="w-1.5 h-1.5 rounded-full bg-blue-500"></div>
                  Detailed request/response data
                </li>
                <li className="flex items-center gap-3 text-zinc-700">
                  <div className="w-1.5 h-1.5 rounded-full bg-blue-500"></div>
                  Custom context & tagging
                </li>
              </ul>
            </div>
            <div className="flex-1 w-full relative">
              <div className="absolute inset-0 bg-gradient-to-tl from-blue-100/50 to-transparent rounded-3xl transform -rotate-3 scale-105 -z-10"></div>
              <div className="relative rounded-xl overflow-hidden border border-zinc-200 shadow-2xl shadow-zinc-200/50 bg-white">
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

          {/* Feature 3: Performance (Segments) */}
          <div className="flex flex-col md:flex-row items-center gap-12 lg:gap-20">
            <div className="flex-1 space-y-6">
              <div className="w-12 h-12 bg-orange-50 rounded-2xl flex items-center justify-center">
                <ChartGantt className="w-6 h-6 text-orange-600" />
              </div>
              <h3 className="text-2xl md:text-3xl font-bold text-zinc-900 tracking-tight">Performance waterfall</h3>
              <p className="text-zinc-600 text-lg leading-relaxed">
                Visualize latency with precision. Our segment waterfall view breaks down every operation in your request,
                showing you exactly which database query or external API call is slowing you down.
              </p>
              <ul className="space-y-3 pt-2">
                <li className="flex items-center gap-3 text-zinc-700">
                  <div className="w-1.5 h-1.5 rounded-full bg-orange-500"></div>
                  Operation-level timing
                </li>
                <li className="flex items-center gap-3 text-zinc-700">
                  <div className="w-1.5 h-1.5 rounded-full bg-orange-500"></div>
                  Identify bottlenecks instantly
                </li>
              </ul>
            </div>
            <div className="flex-1 w-full relative">
              <div className="absolute inset-0 bg-gradient-to-tr from-orange-100/50 to-transparent rounded-3xl transform rotate-3 scale-105 -z-10"></div>
              <div className="relative rounded-xl overflow-hidden border border-zinc-200 shadow-2xl shadow-zinc-200/50 bg-white">
                <Image
                  src="/images/screenshot-3.png"
                  alt="Performance Waterfall"
                  width={800}
                  height={600}
                  className="w-full h-auto"
                />
              </div>
            </div>
          </div>

        </div>
      </section>

      {/* Q&A Section */}
      <section className="py-24 bg-zinc-50 border-t border-zinc-100">
        <div className="container mx-auto px-4 max-w-3xl">
          <div className="text-center mb-12">
            <h2 className="text-3xl font-bold mb-4 text-zinc-900 tracking-tight">Frequently Asked Questions</h2>
            <p className="text-zinc-600 text-lg">
              Everything you need to know about Traceway.
            </p>
          </div>

          <Accordion type="single" collapsible className="w-full">
            <AccordionItem value="item-1" className="border-b-zinc-200">
              <AccordionTrigger className="text-zinc-900 hover:text-zinc-700 hover:no-underline text-left">
                What is Traceway?
              </AccordionTrigger>
              <AccordionContent className="text-zinc-600 leading-relaxed">
                Traceway is an open-source telemetry and issue tracking platform. It is designed for Golang applications.
                It helps prioritize tasks, track issues and optimize performance by providing exception tracking,
                performance insights, and regression monitoring.
              </AccordionContent>
            </AccordionItem>
            <AccordionItem value="item-2" className="border-b-zinc-200">
              <AccordionTrigger className="text-zinc-900 hover:text-zinc-700 hover:no-underline text-left">
                How does it compare to OpenTelemetry?
              </AccordionTrigger>
              <AccordionContent className="text-zinc-600 leading-relaxed">
                While OpenTelemetry is a powerful, generalized standard, it can be complex to configure and manage.
                Traceway is "batteries-included" and opinionated, focusing on immediate value for Go developers
                without the configuration overhead. We provide actionable insights (like issue ranking) out of the box.
                It has a simple deployment model that makes it cheap to run in production.
              </AccordionContent>
            </AccordionItem>
            <AccordionItem value="item-3" className="border-b-zinc-200">
              <AccordionTrigger className="text-zinc-900 hover:text-zinc-700 hover:no-underline text-left">
                How does it compare to Sentry?
              </AccordionTrigger>
              <AccordionContent className="text-zinc-600 leading-relaxed">
                Sentry is a great tool, but it can be expensive and their performance tracking for Golang applications is not as good as Traceway. Traceway is a lightweight,
                open-source alternative that is easy to use and deploy. It is also more affordable to run than Sentry. Traceway offers more features for Golang applications specifically focusing on performance.
              </AccordionContent>
            </AccordionItem>
            <AccordionItem value="item-4" className="border-b-zinc-200">
              <AccordionTrigger className="text-zinc-900 hover:text-zinc-700 hover:no-underline text-left">
                How does it compare to New Relic?
              </AccordionTrigger>
              <AccordionContent className="text-zinc-600 leading-relaxed">
                New Relic is incredible for raw data tracking but due to their wide feature set it can be hard to navigate and find what you actually "need" to fix.
                Traceway addresses this by focusing on telling you what needs fixing by prioritizing and ranking endpoints. It provides server metrics (similar to new relic) while also providing a great issue tracking solution like Sentry.
              </AccordionContent>
            </AccordionItem>
            <AccordionItem value="item-5" className="border-b-zinc-200">
              <AccordionTrigger className="text-zinc-900 hover:text-zinc-700 hover:no-underline text-left">
                Can I self-host Traceway?
              </AccordionTrigger>
              <AccordionContent className="text-zinc-600 leading-relaxed">
                Yes! Traceway is open source and can be easily self-hosted. We provide Docker containers
                and binaries to make running your own instance straightforward.
              </AccordionContent>
            </AccordionItem>
            <AccordionItem value="item-6" className="border-b-zinc-200">
              <AccordionTrigger className="text-zinc-900 hover:text-zinc-700 hover:no-underline text-left">
                Is there a performance impact on the client?
              </AccordionTrigger>
              <AccordionContent className="text-zinc-600 leading-relaxed">
                We designed the Traceway agent to be extremely lightweight. It uses efficient batching and
                asynchronous reporting to ensure it has negligible impact on your application's latency or throughput.
              </AccordionContent>
            </AccordionItem>
          </Accordion>
        </div>
      </section>

      {/* Footer */}
      <footer className="py-8 border-t border-zinc-200 bg-white">
        <div className="container mx-auto px-4 flex flex-col md:flex-row items-center justify-between text-zinc-500 text-xs">
          <div className="font-medium">
            &copy; {new Date().getFullYear()} Traceway. All rights reserved.
          </div>
          <div className="flex items-center gap-6 mt-3 md:mt-0 font-medium">
            <Link href="https://docs.tracewayapp.com" className="hover:text-zinc-900 transition-colors">
              Docs
            </Link>
            <Link href="https://github.com/tracewayapp/traceway" className="hover:text-zinc-900 transition-colors">
              GitHub
            </Link>
          </div>
        </div>
      </footer>
    </div>
  );
}
