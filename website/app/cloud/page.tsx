import { PricingCalculator } from "@/components/pricing-calculator";
import Link from "next/link";
import { Button } from "@/components/ui/button";
import { ArrowRight } from "lucide-react";
import { Accordion, AccordionContent, AccordionItem, AccordionTrigger } from "@/components/ui/accordion";
import { Badge } from "@/components/ui/badge";

export default function CloudPage() {
    return (
        <main className="min-h-screen bg-white text-zinc-950 font-sans selection:bg-zinc-100 selection:text-zinc-900">
            {/* Hero Section */}
            <section className="relative pt-16 pb-20 overflow-hidden">
                <div className="absolute inset-0 -z-10 h-full w-full bg-white bg-[radial-gradient(#e5e7eb_1px,transparent_1px)] [background-size:16px_16px] [mask-image:radial-gradient(ellipse_50%_50%_at_50%_50%,#000_70%,transparent_100%)]"></div>
                <div className="container mx-auto px-4 text-center">
                    <Badge variant="secondary" className="mb-4 bg-blue-50 text-blue-700 hover:bg-blue-100 px-2.5 py-0.5 border border-blue-100 text-xs font-normal rounded-full">
                        Traceway Cloud
                    </Badge>
                    <h1 className="text-4xl md:text-6xl font-bold tracking-tight mb-6 text-zinc-900">
                        Managed Traceway <span className="text-transparent bg-clip-text bg-gradient-to-r from-blue-600 to-indigo-600">for Teams</span>
                    </h1>
                    <p className="text-zinc-600 text-lg md:text-xl max-w-xl mx-auto mb-10 leading-relaxed font-medium">
                        Focus on shipping features, not managing infrastructure. Get all the power of Traceway with zero maintenance.
                    </p>
                    <div className="flex flex-col sm:flex-row items-center justify-center gap-3">
                        <Link href="http://cloud.tracewayapp.com/register">
                            <Button size="lg" className="h-10 px-6 text-sm bg-[#4ba3f7] text-white hover:bg-[#3b93e7] font-bold shadow-sm shadow-blue-400/20">
                                Start Free Trial <ArrowRight className="ml-2 h-4 w-4" />
                            </Button>
                        </Link>
                        <Link href="https://docs.tracewayapp.com/cloud">
                            <Button variant="outline" size="lg" className="h-10 px-6 text-sm border-zinc-200 bg-white hover:bg-zinc-50 text-zinc-900">
                                How it works
                            </Button>
                        </Link>
                    </div>
                </div>
            </section>

            {/* Pricing Section */}
            <section className="py-24 bg-zinc-50/50 border-y border-zinc-100">
                <div className="container mx-auto px-4 max-w-5xl">
                    <div className="text-center mb-16">
                        <h2 className="text-3xl md:text-4xl font-bold mb-4 text-zinc-900 tracking-tight">Simple, usage-based pricing</h2>
                        <p className="text-zinc-600 text-lg max-w-xl mx-auto">
                            Start for free and scale as you grow. No credit card required for the starter plan.
                        </p>
                    </div>

                    <PricingCalculator />
                </div>
            </section>

            {/* Cloud vs Self-Hosted Q&A */}
            <section className="py-24 bg-white">
                <div className="container mx-auto px-4 max-w-3xl">
                    <div className="text-center mb-12">
                        <h2 className="text-3xl font-bold mb-4 text-zinc-900 tracking-tight">Cloud vs. Self-Hosted</h2>
                        <p className="text-zinc-600 text-lg">
                            Common questions about our deployment options.
                        </p>
                    </div>

                    <Accordion type="single" collapsible className="w-full">
                        <AccordionItem value="item-1" className="border-b-zinc-200">
                            <AccordionTrigger className="text-zinc-900 hover:text-zinc-700 hover:no-underline text-left">
                                Why use Traceway Cloud?
                            </AccordionTrigger>
                            <AccordionContent className="text-zinc-600 leading-relaxed">
                                Traceway Cloud is simply for teams that don't want to self-host. We run the exact same open-source code but manage the infrastructure, updates, and backups for you.
                                It allows you to focus on shipping features without worrying about maintaining an observability stack.
                            </AccordionContent>
                        </AccordionItem>
                        <AccordionItem value="item-2" className="border-b-zinc-200">
                            <AccordionTrigger className="text-zinc-900 hover:text-zinc-700 hover:no-underline text-left">
                                Is the Open Source version limited?
                            </AccordionTrigger>
                            <AccordionContent className="text-zinc-600 leading-relaxed">
                                No. The code is 100% open source and fully featured. We do not gate features behind the cloud version.
                                The cloud offering exists solely for convenience and for users who prefer a managed service over self-hosting.
                            </AccordionContent>
                        </AccordionItem>
                        <AccordionItem value="item-3" className="border-b-zinc-200">
                            <AccordionTrigger className="text-zinc-900 hover:text-zinc-700 hover:no-underline text-left">
                                Can I migrate from Cloud to Self-Hosted later?
                            </AccordionTrigger>
                            <AccordionContent className="text-zinc-600 leading-relaxed">
                                Yes, since the underlying software is the same, we can work with you to export your data and migrate to a self-hosted instance at any time.
                                You are never locked into our cloud platform.
                            </AccordionContent>
                        </AccordionItem>
                    </Accordion>
                </div>
            </section>
        </main>
    );
}
