import Image from "next/image";
import Link from "next/link";

import { Badge } from "@/components/ui/badge";
import { Accordion, AccordionContent, AccordionItem, AccordionTrigger } from "@/components/ui/accordion";
import { ArrowRight, Video, Zap, ShieldCheck } from "lucide-react";

export default function SessionReplayPage() {
    return (
        <main className="min-h-screen bg-white text-zinc-950 font-sans selection:bg-zinc-100 selection:text-zinc-900">
            {/* Hero Section */}
            <section className="relative pt-16 pb-20 overflow-hidden">
                <div className="absolute inset-0 -z-10 h-full w-full bg-white bg-[radial-gradient(#e5e7eb_1px,transparent_1px)] [background-size:16px_16px] [mask-image:radial-gradient(ellipse_50%_50%_at_50%_50%,#000_70%,transparent_100%)]"></div>
                <div className="container mx-auto px-4 text-center">
                    <Badge variant="secondary" className="mb-4 bg-purple-50 text-purple-700 hover:bg-purple-100 px-2.5 py-0.5 border border-purple-100 text-xs font-normal rounded-full">
                        Session Replay
                    </Badge>
                    <h1 className="text-4xl md:text-6xl font-bold tracking-tight mb-6 text-zinc-900">
                        See what caused <br /> <span className="text-transparent bg-clip-text bg-gradient-to-r from-purple-600 to-violet-600">every error</span>
                    </h1>
                    <p className="text-zinc-600 text-lg md:text-xl max-w-2xl mx-auto mb-10 leading-relaxed font-medium">
                        Skip the log-digging and reproduction steps. Watch the user&apos;s actual clicks, scrolls, and navigations leading up to every exception.
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

            {/* Watch the moments before every error */}
            <section className="py-24 bg-white border-y border-zinc-100">
                <div className="container mx-auto px-4 max-w-5xl">
                    <div className="flex flex-col md:flex-row items-center gap-12 lg:gap-20">
                        <div className="flex-1 space-y-6">
                            <div className="w-12 h-12 bg-purple-50 rounded-2xl flex items-center justify-center">
                                <Video className="w-6 h-6 text-purple-600" />
                            </div>
                            <h2 className="text-2xl md:text-3xl font-bold text-zinc-900 tracking-tight">Watch the moments before every error</h2>
                            <p className="text-zinc-600 text-lg leading-relaxed">
                                Traceway captures user activity leading up to exceptions - clicks, scrolls, form fills, and page navigations.
                                When something breaks, you see exactly what happened.
                            </p>
                            <ul className="space-y-3 pt-2">
                                <li className="flex items-center gap-3 text-zinc-700">
                                    <div className="w-1.5 h-1.5 rounded-full bg-purple-500"></div>
                                    Pre-error activity capture
                                </li>
                                <li className="flex items-center gap-3 text-zinc-700">
                                    <div className="w-1.5 h-1.5 rounded-full bg-purple-500"></div>
                                    Clicks, scrolls, and form interactions
                                </li>
                                <li className="flex items-center gap-3 text-zinc-700">
                                    <div className="w-1.5 h-1.5 rounded-full bg-purple-500"></div>
                                    Page navigation timeline
                                </li>
                            </ul>
                        </div>
                        <div className="flex-1 w-full relative">
                            <div className="absolute inset-0 bg-gradient-to-tr from-purple-100/50 to-transparent rounded-3xl transform rotate-3 scale-105 -z-10"></div>
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
                </div>
            </section>

            {/* Attached to every exception + Built for privacy */}
            <section className="py-24 bg-zinc-50/50">
                <div className="container mx-auto px-4 max-w-5xl">
                    <div className="grid grid-cols-1 md:grid-cols-2 gap-8">
                        <div className="rounded-2xl border border-zinc-200 bg-white p-8 space-y-5">
                            <div className="w-12 h-12 bg-orange-50 rounded-2xl flex items-center justify-center">
                                <Zap className="w-6 h-6 text-orange-600" />
                            </div>
                            <h2 className="text-xl md:text-2xl font-bold text-zinc-900 tracking-tight">Attached to every exception automatically</h2>
                            <p className="text-zinc-600 leading-relaxed">
                                No manual setup or reproduction steps. Every exception gets a replay attached automatically.
                                Click into any issue and watch exactly what the user did.
                            </p>
                            <ul className="space-y-3 pt-1">
                                <li className="flex items-center gap-3 text-zinc-700">
                                    <div className="w-1.5 h-1.5 rounded-full bg-orange-500"></div>
                                    Zero-config capture
                                </li>
                                <li className="flex items-center gap-3 text-zinc-700">
                                    <div className="w-1.5 h-1.5 rounded-full bg-orange-500"></div>
                                    Linked directly to exceptions
                                </li>
                                <li className="flex items-center gap-3 text-zinc-700">
                                    <div className="w-1.5 h-1.5 rounded-full bg-orange-500"></div>
                                    No reproduction needed
                                </li>
                            </ul>
                        </div>

                        <div className="rounded-2xl border border-zinc-200 bg-white p-8 space-y-5">
                            <div className="w-12 h-12 bg-green-50 rounded-2xl flex items-center justify-center">
                                <ShieldCheck className="w-6 h-6 text-green-600" />
                            </div>
                            <h2 className="text-xl md:text-2xl font-bold text-zinc-900 tracking-tight">Built for privacy</h2>
                            <p className="text-zinc-600 leading-relaxed">
                                Sensitive inputs are masked by default. Passwords, credit cards, and personal data are never recorded.
                                You see the interaction flow, not the private content.
                            </p>
                            <ul className="space-y-3 pt-1">
                                <li className="flex items-center gap-3 text-zinc-700">
                                    <div className="w-1.5 h-1.5 rounded-full bg-green-500"></div>
                                    Sensitive input masking
                                </li>
                                <li className="flex items-center gap-3 text-zinc-700">
                                    <div className="w-1.5 h-1.5 rounded-full bg-green-500"></div>
                                    No passwords or payment data recorded
                                </li>
                                <li className="flex items-center gap-3 text-zinc-700">
                                    <div className="w-1.5 h-1.5 rounded-full bg-green-500"></div>
                                    Compliant by default
                                </li>
                            </ul>
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
                            Common questions about session replay with Traceway.
                        </p>
                    </div>

                    <Accordion type="single" collapsible className="w-full">
                        <AccordionItem value="item-1" className="border-b-zinc-200">
                            <AccordionTrigger className="text-zinc-900 hover:text-zinc-700 hover:no-underline text-left">
                                How does session replay work?
                            </AccordionTrigger>
                            <AccordionContent className="text-zinc-600 leading-relaxed">
                                Traceway records DOM changes in the browser. When an exception occurs, approximately 10 seconds of user activity
                                is captured and attached to the error automatically.
                            </AccordionContent>
                        </AccordionItem>
                        <AccordionItem value="item-2" className="border-b-zinc-200">
                            <AccordionTrigger className="text-zinc-900 hover:text-zinc-700 hover:no-underline text-left">
                                Does session replay affect performance?
                            </AccordionTrigger>
                            <AccordionContent className="text-zinc-600 leading-relaxed">
                                The recording adds minimal overhead. Captures run in the background and only the relevant buffer is sent when an error occurs.
                            </AccordionContent>
                        </AccordionItem>
                        <AccordionItem value="item-3" className="border-b-zinc-200">
                            <AccordionTrigger className="text-zinc-900 hover:text-zinc-700 hover:no-underline text-left">
                                What about user privacy?
                            </AccordionTrigger>
                            <AccordionContent className="text-zinc-600 leading-relaxed">
                                Sensitive fields are masked by default. Passwords, credit card numbers, and other private inputs are never captured.
                            </AccordionContent>
                        </AccordionItem>
                    </Accordion>
                </div>
            </section>
        </main>
    );
}
