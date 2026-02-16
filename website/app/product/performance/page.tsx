import Image from "next/image";
import Link from "next/link";

import { Badge } from "@/components/ui/badge";
import { Accordion, AccordionContent, AccordionItem, AccordionTrigger } from "@/components/ui/accordion";
import { ArrowRight, Activity, ChartGantt, Cpu } from "lucide-react";

export default function PerformancePage() {
    return (
        <main className="min-h-screen bg-white text-zinc-950 font-sans selection:bg-zinc-100 selection:text-zinc-900">
            {/* Hero Section */}
            <section className="relative pt-16 pb-20 overflow-hidden">
                <div className="absolute inset-0 -z-10 h-full w-full bg-white bg-[radial-gradient(#e5e7eb_1px,transparent_1px)] [background-size:16px_16px] [mask-image:radial-gradient(ellipse_50%_50%_at_50%_50%,#000_70%,transparent_100%)]"></div>
                <div className="container mx-auto px-4 text-center">
                    <Badge variant="secondary" className="mb-4 bg-blue-50 text-blue-700 hover:bg-blue-100 px-2.5 py-0.5 border border-blue-100 text-xs font-normal rounded-full">
                        Performance
                    </Badge>
                    <h1 className="text-4xl md:text-6xl font-bold tracking-tight mb-6 text-zinc-900">
                        Understand exactly <br /> <span className="text-transparent bg-clip-text bg-gradient-to-r from-blue-600 to-indigo-600">where time is spent</span>
                    </h1>
                    <p className="text-zinc-600 text-lg md:text-xl max-w-2xl mx-auto mb-10 leading-relaxed font-medium">
                        P50/P95/P99 percentiles for every endpoint, span waterfall traces, and real-time server metrics. Know exactly what&apos;s slow and why.
                    </p>
                    <div className="flex flex-col sm:flex-row items-center justify-center gap-3">
                        <Link href="https://docs.tracewayapp.com" className="inline-flex items-center justify-center gap-2 rounded-md text-sm font-medium transition-all cursor-pointer h-10 px-6 bg-zinc-900 text-white hover:bg-zinc-800 shadow-lg shadow-zinc-900/20">
                                Get Started <ArrowRight className="ml-2 h-4 w-4" />
                        </Link>
                        <Link href="http://cloud.tracewayapp.com/register" className="inline-flex items-center justify-center gap-2 rounded-md text-sm font-medium transition-all cursor-pointer h-10 px-6 border border-zinc-200 bg-white hover:bg-zinc-50 text-zinc-900 shadow-sm">
                                Try Traceway Cloud
                        </Link>
                    </div>
                </div>
            </section>

            {/* Endpoint Analytics Section */}
            <section className="py-24 bg-white border-y border-zinc-100">
                <div className="container mx-auto px-4 max-w-5xl">
                    <div className="flex flex-col md:flex-row items-center gap-12 lg:gap-20">
                        <div className="flex-1 space-y-6">
                            <div className="w-12 h-12 bg-blue-50 rounded-2xl flex items-center justify-center">
                                <Activity className="w-6 h-6 text-blue-600" />
                            </div>
                            <h2 className="text-2xl md:text-3xl font-bold text-zinc-900 tracking-tight">Endpoint analytics at a glance</h2>
                            <p className="text-zinc-600 text-lg leading-relaxed">
                                See P50, P95, and P99 percentiles for every endpoint in your application. Quickly identify which routes
                                are fast and which need attention, with throughput and error rate breakdowns.
                            </p>
                            <ul className="space-y-3 pt-2">
                                <li className="flex items-center gap-3 text-zinc-700">
                                    <div className="w-1.5 h-1.5 rounded-full bg-blue-500"></div>
                                    P50/P95/P99 latency percentiles
                                </li>
                                <li className="flex items-center gap-3 text-zinc-700">
                                    <div className="w-1.5 h-1.5 rounded-full bg-blue-500"></div>
                                    Throughput and error rate
                                </li>
                                <li className="flex items-center gap-3 text-zinc-700">
                                    <div className="w-1.5 h-1.5 rounded-full bg-blue-500"></div>
                                    Historical trend comparison
                                </li>
                            </ul>
                        </div>
                        <div className="flex-1 w-full relative">
                            <div className="absolute inset-0 bg-gradient-to-tr from-blue-100/50 to-transparent rounded-3xl transform rotate-3 scale-105 -z-10"></div>
                            <div className="relative rounded-xl overflow-hidden border border-zinc-200 bg-white">
                                <Image
                                    src="/images/screenshot-1.png"
                                    alt="Endpoint Analytics"
                                    width={800}
                                    height={600}
                                    className="w-full h-auto"
                                />
                            </div>
                        </div>
                    </div>
                </div>
            </section>

            {/* Waterfall Section */}
            <section className="py-24 bg-zinc-50/50">
                <div className="container mx-auto px-4 max-w-5xl">
                    <div className="flex flex-col md:flex-row-reverse items-center gap-12 lg:gap-20">
                        <div className="flex-1 space-y-6">
                            <div className="w-12 h-12 bg-orange-50 rounded-2xl flex items-center justify-center">
                                <ChartGantt className="w-6 h-6 text-orange-600" />
                            </div>
                            <h2 className="text-2xl md:text-3xl font-bold text-zinc-900 tracking-tight">Span waterfall view</h2>
                            <p className="text-zinc-600 text-lg leading-relaxed">
                                Break down every request into its component operations. See exactly which database query,
                                external API call, or middleware is adding latency, with precise timing for each span.
                            </p>
                            <ul className="space-y-3 pt-2">
                                <li className="flex items-center gap-3 text-zinc-700">
                                    <div className="w-1.5 h-1.5 rounded-full bg-orange-500"></div>
                                    Operation-level timing breakdown
                                </li>
                                <li className="flex items-center gap-3 text-zinc-700">
                                    <div className="w-1.5 h-1.5 rounded-full bg-orange-500"></div>
                                    Visual waterfall timeline
                                </li>
                                <li className="flex items-center gap-3 text-zinc-700">
                                    <div className="w-1.5 h-1.5 rounded-full bg-orange-500"></div>
                                    Pinpoint bottlenecks instantly
                                </li>
                            </ul>
                        </div>
                        <div className="flex-1 w-full relative">
                            <div className="absolute inset-0 bg-gradient-to-tl from-orange-100/50 to-transparent rounded-3xl transform -rotate-3 scale-105 -z-10"></div>
                            <div className="relative rounded-xl overflow-hidden border border-zinc-200 bg-white">
                                <Image
                                    src="/images/screenshot-3.png"
                                    alt="Span Waterfall View"
                                    width={800}
                                    height={600}
                                    className="w-full h-auto"
                                />
                            </div>
                        </div>
                    </div>
                </div>
            </section>

            {/* Server Metrics Section */}
            <section className="py-24 bg-white border-y border-zinc-100">
                <div className="container mx-auto px-4 max-w-5xl">
                    <div className="flex flex-col md:flex-row items-center gap-12 lg:gap-20">
                        <div className="flex-1 space-y-6">
                            <div className="w-12 h-12 bg-green-50 rounded-2xl flex items-center justify-center">
                                <Cpu className="w-6 h-6 text-green-600" />
                            </div>
                            <h2 className="text-2xl md:text-3xl font-bold text-zinc-900 tracking-tight">Server metrics in real time</h2>
                            <p className="text-zinc-600 text-lg leading-relaxed">
                                Monitor CPU usage, memory consumption, goroutines, and garbage collection alongside your
                                application telemetry. Correlate infrastructure health with endpoint performance.
                            </p>
                            <ul className="space-y-3 pt-2">
                                <li className="flex items-center gap-3 text-zinc-700">
                                    <div className="w-1.5 h-1.5 rounded-full bg-green-500"></div>
                                    CPU and memory monitoring
                                </li>
                                <li className="flex items-center gap-3 text-zinc-700">
                                    <div className="w-1.5 h-1.5 rounded-full bg-green-500"></div>
                                    Go runtime metrics (goroutines, GC)
                                </li>
                                <li className="flex items-center gap-3 text-zinc-700">
                                    <div className="w-1.5 h-1.5 rounded-full bg-green-500"></div>
                                    Automatic collection via SDK
                                </li>
                            </ul>
                        </div>
                        <div className="flex-1 w-full relative">
                            <div className="absolute inset-0 bg-gradient-to-tr from-green-100/50 to-transparent rounded-3xl transform rotate-3 scale-105 -z-10"></div>
                            <div className="relative rounded-xl overflow-hidden border border-zinc-200 bg-white">
                                <Image
                                    src="/images/screenshot-4.png"
                                    alt="Server Metrics Dashboard"
                                    width={800}
                                    height={600}
                                    className="w-full h-auto"
                                />
                            </div>
                        </div>
                    </div>
                </div>
            </section>

            {/* FAQ Section */}
            <section className="py-24 bg-zinc-50 border-t border-zinc-100">
                <div className="container mx-auto px-4 max-w-3xl">
                    <div className="text-center mb-12">
                        <h2 className="text-3xl font-bold mb-4 text-zinc-900 tracking-tight">Frequently Asked Questions</h2>
                        <p className="text-zinc-600 text-lg">
                            Common questions about performance monitoring with Traceway.
                        </p>
                    </div>

                    <Accordion type="single" collapsible className="w-full">
                        <AccordionItem value="item-1" className="border-b-zinc-200">
                            <AccordionTrigger className="text-zinc-900 hover:text-zinc-700 hover:no-underline text-left">
                                What percentiles does Traceway track?
                            </AccordionTrigger>
                            <AccordionContent className="text-zinc-600 leading-relaxed">
                                Traceway calculates P50 (median), P95, and P99 latency percentiles for every endpoint.
                                This gives you a clear picture of both typical and worst-case response times for your application.
                            </AccordionContent>
                        </AccordionItem>
                        <AccordionItem value="item-2" className="border-b-zinc-200">
                            <AccordionTrigger className="text-zinc-900 hover:text-zinc-700 hover:no-underline text-left">
                                How does the waterfall view work?
                            </AccordionTrigger>
                            <AccordionContent className="text-zinc-600 leading-relaxed">
                                The waterfall view shows every span (database query, external API call, etc.) within a single request as a timeline.
                                You can see how long each operation took and where they overlap, making it easy to identify the slowest part of any request.
                            </AccordionContent>
                        </AccordionItem>
                        <AccordionItem value="item-3" className="border-b-zinc-200">
                            <AccordionTrigger className="text-zinc-900 hover:text-zinc-700 hover:no-underline text-left">
                                What server metrics are collected automatically?
                            </AccordionTrigger>
                            <AccordionContent className="text-zinc-600 leading-relaxed">
                                The Traceway SDK automatically collects CPU usage, memory usage, active goroutine count, heap object count,
                                GC cycle count, and GC pause time. No additional configuration is required&mdash;just add the middleware and metrics start flowing.
                            </AccordionContent>
                        </AccordionItem>
                    </Accordion>
                </div>
            </section>
        </main>
    );
}
