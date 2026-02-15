import Image from "next/image";
import Link from "next/link";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Accordion, AccordionContent, AccordionItem, AccordionTrigger } from "@/components/ui/accordion";
import { ArrowRight, TrendingUp, Layers, Globe } from "lucide-react";

const frameworks = [
    { name: "Gin", src: "/images/frameworks/gin.png" },
    { name: "Chi", src: "/images/frameworks/chi.png" },
    { name: "Express", src: "/images/frameworks/express.png" },
    { name: "NestJS", src: "/images/frameworks/nestjs.png" },
    { name: "Next.js", src: "/images/frameworks/nextjs.png" },
    { name: "Svelte", src: "/images/frameworks/svelte.png" },
    { name: "Remix", src: "/images/frameworks/remix.png" },
    { name: "OpenTelemetry", src: "/images/frameworks/otel.png" },
];

export default function IssueTrackingPage() {
    return (
        <main className="min-h-screen bg-white text-zinc-950 font-sans selection:bg-zinc-100 selection:text-zinc-900">
            {/* Hero Section */}
            <section className="relative pt-16 pb-20 overflow-hidden">
                <div className="absolute inset-0 -z-10 h-full w-full bg-white bg-[radial-gradient(#e5e7eb_1px,transparent_1px)] [background-size:16px_16px] [mask-image:radial-gradient(ellipse_50%_50%_at_50%_50%,#000_70%,transparent_100%)]"></div>
                <div className="container mx-auto px-4 text-center">
                    <Badge variant="secondary" className="mb-4 bg-red-50 text-red-700 hover:bg-red-100 px-2.5 py-0.5 border border-red-100 text-xs font-normal rounded-full">
                        Issue Tracking
                    </Badge>
                    <h1 className="text-4xl md:text-6xl font-bold tracking-tight mb-6 text-zinc-900">
                        Find and fix issues <br /> <span className="text-transparent bg-clip-text bg-gradient-to-r from-red-600 to-orange-600">before your users notice</span>
                    </h1>
                    <p className="text-zinc-600 text-lg md:text-xl max-w-2xl mx-auto mb-10 leading-relaxed font-medium">
                        Traceway automatically ranks issues by impact across your Go and JavaScript services, so your team always knows what to fix first.
                    </p>
                    <div className="flex flex-col sm:flex-row items-center justify-center gap-3">
                        <Link href="https://docs.tracewayapp.com">
                            <Button size="lg" className="h-10 px-6 text-sm bg-zinc-900 text-white hover:bg-zinc-800 shadow-lg shadow-zinc-900/20">
                                Get Started <ArrowRight className="ml-2 h-4 w-4" />
                            </Button>
                        </Link>
                        <Link href="http://cloud.tracewayapp.com/register">
                            <Button variant="outline" size="lg" className="h-10 px-6 text-sm border-zinc-200 bg-white hover:bg-zinc-50 text-zinc-900 shadow-sm">
                                Try Traceway Cloud
                            </Button>
                        </Link>
                    </div>
                </div>
            </section>

            {/* Automatic Ranking Section */}
            <section className="py-24 bg-white border-y border-zinc-100">
                <div className="container mx-auto px-4 max-w-5xl">
                    <div className="flex flex-col md:flex-row items-center gap-12 lg:gap-20">
                        <div className="flex-1 space-y-6">
                            <div className="w-12 h-12 bg-blue-50 rounded-2xl flex items-center justify-center">
                                <TrendingUp className="w-6 h-6 text-blue-600" />
                            </div>
                            <h2 className="text-2xl md:text-3xl font-bold text-zinc-900 tracking-tight">Issues ranked by what matters</h2>
                            <p className="text-zinc-600 text-lg leading-relaxed">
                                Stop triaging manually. Traceway ranks every issue by frequency, user impact, and recency so
                                your team focuses on the problems that matter most. New regressions surface immediately.
                            </p>
                            <ul className="space-y-3 pt-2">
                                <li className="flex items-center gap-3 text-zinc-700">
                                    <div className="w-1.5 h-1.5 rounded-full bg-blue-500"></div>
                                    Impact-based ranking
                                </li>
                                <li className="flex items-center gap-3 text-zinc-700">
                                    <div className="w-1.5 h-1.5 rounded-full bg-blue-500"></div>
                                    Regression detection
                                </li>
                                <li className="flex items-center gap-3 text-zinc-700">
                                    <div className="w-1.5 h-1.5 rounded-full bg-blue-500"></div>
                                    Frequency and recency scoring
                                </li>
                            </ul>
                        </div>
                        <div className="flex-1 w-full relative">
                            <div className="absolute inset-0 bg-gradient-to-tr from-blue-100/50 to-transparent rounded-3xl transform rotate-3 scale-105 -z-10"></div>
                            <div className="relative rounded-xl overflow-hidden border border-zinc-200 bg-white">
                                <Image
                                    src="/images/screenshot-2.png"
                                    alt="Issue Ranking Dashboard"
                                    width={800}
                                    height={600}
                                    className="w-full h-auto"
                                />
                            </div>
                        </div>
                    </div>
                </div>
            </section>

            {/* Error Grouping Section */}
            <section className="py-24 bg-zinc-50/50">
                <div className="container mx-auto px-4 max-w-5xl">
                    <div className="flex flex-col md:flex-row-reverse items-center gap-12 lg:gap-20">
                        <div className="flex-1 space-y-6">
                            <div className="w-12 h-12 bg-orange-50 rounded-2xl flex items-center justify-center">
                                <Layers className="w-6 h-6 text-orange-600" />
                            </div>
                            <h2 className="text-2xl md:text-3xl font-bold text-zinc-900 tracking-tight">Intelligent error grouping</h2>
                            <p className="text-zinc-600 text-lg leading-relaxed">
                                Traceway normalizes stack traces before hashing, so the same logical error gets grouped together
                                even when runtime values differ. No more duplicate issues cluttering your dashboard.
                            </p>
                            <ul className="space-y-3 pt-2">
                                <li className="flex items-center gap-3 text-zinc-700">
                                    <div className="w-1.5 h-1.5 rounded-full bg-orange-500"></div>
                                    Stack trace normalization
                                </li>
                                <li className="flex items-center gap-3 text-zinc-700">
                                    <div className="w-1.5 h-1.5 rounded-full bg-orange-500"></div>
                                    Cross-service deduplication
                                </li>
                                <li className="flex items-center gap-3 text-zinc-700">
                                    <div className="w-1.5 h-1.5 rounded-full bg-orange-500"></div>
                                    Full context on every occurrence
                                </li>
                            </ul>
                        </div>
                        <div className="flex-1 w-full relative">
                            <div className="absolute inset-0 bg-gradient-to-tl from-orange-100/50 to-transparent rounded-3xl transform -rotate-3 scale-105 -z-10"></div>
                            <div className="relative rounded-xl overflow-hidden border border-zinc-200 bg-white">
                                <Image
                                    src="/images/screenshot-2.png"
                                    alt="Error Grouping Interface"
                                    width={800}
                                    height={600}
                                    className="w-full h-auto"
                                />
                            </div>
                        </div>
                    </div>
                </div>
            </section>

            {/* Full Stack Section */}
            <section className="py-24 bg-white border-y border-zinc-100">
                <div className="container mx-auto px-4 max-w-5xl">
                    <div className="flex flex-col items-center text-center space-y-6">
                        <div className="w-12 h-12 bg-green-50 rounded-2xl flex items-center justify-center">
                            <Globe className="w-6 h-6 text-green-600" />
                        </div>
                        <h2 className="text-2xl md:text-3xl font-bold text-zinc-900 tracking-tight">Track issues across your full stack</h2>
                        <p className="text-zinc-600 text-lg leading-relaxed max-w-2xl">
                            From Go backends to JavaScript frontends, Traceway captures exceptions everywhere your code runs.
                            Get a unified view of issues across services, with full stack traces and contextual tags.
                        </p>
                        <div className="flex flex-wrap items-center justify-center gap-8 md:gap-10 pt-6">
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
                </div>
            </section>

            {/* FAQ Section */}
            <section className="py-24 bg-zinc-50 border-t border-zinc-100">
                <div className="container mx-auto px-4 max-w-3xl">
                    <div className="text-center mb-12">
                        <h2 className="text-3xl font-bold mb-4 text-zinc-900 tracking-tight">Frequently Asked Questions</h2>
                        <p className="text-zinc-600 text-lg">
                            Common questions about issue tracking with Traceway.
                        </p>
                    </div>

                    <Accordion type="single" collapsible className="w-full">
                        <AccordionItem value="item-1" className="border-b-zinc-200">
                            <AccordionTrigger className="text-zinc-900 hover:text-zinc-700 hover:no-underline text-left">
                                How does automatic issue ranking work?
                            </AccordionTrigger>
                            <AccordionContent className="text-zinc-600 leading-relaxed">
                                Traceway scores each issue based on how often it occurs, how recently it appeared, and how many users are affected.
                                Issues are continuously re-ranked as new data comes in, so regressions and trending problems surface immediately.
                            </AccordionContent>
                        </AccordionItem>
                        <AccordionItem value="item-2" className="border-b-zinc-200">
                            <AccordionTrigger className="text-zinc-900 hover:text-zinc-700 hover:no-underline text-left">
                                How does error grouping handle different environments?
                            </AccordionTrigger>
                            <AccordionContent className="text-zinc-600 leading-relaxed">
                                Traceway normalizes stack traces by removing runtime-specific values like memory addresses, file paths, UUIDs, and timestamps
                                before hashing. This means the same bug produces the same group regardless of which server or environment it occurred on.
                            </AccordionContent>
                        </AccordionItem>
                        <AccordionItem value="item-3" className="border-b-zinc-200">
                            <AccordionTrigger className="text-zinc-900 hover:text-zinc-700 hover:no-underline text-left">
                                Can I track frontend JavaScript errors?
                            </AccordionTrigger>
                            <AccordionContent className="text-zinc-600 leading-relaxed">
                                Yes. Traceway supports frontend frameworks like Next.js, Svelte, and Remix alongside backend frameworks like Express and NestJS.
                                Errors from both your frontend and backend appear in the same dashboard with full stack traces.
                            </AccordionContent>
                        </AccordionItem>
                    </Accordion>
                </div>
            </section>
        </main>
    );
}
