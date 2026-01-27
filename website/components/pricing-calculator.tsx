"use client";

import * as React from "react";
import { cn } from "@/lib/utils";
import Link from "next/link";
import { ArrowRight } from "lucide-react";
import { Button } from "@/components/ui/button";

const TIERS = [
    {
        id: "starter",
        name: "Starter",
        limit: "10k",
        price: "Free",
        description: "10k issues, requests, task runs",
    },
    {
        id: "pro",
        name: "Pro",
        limit: "100k",
        price: "$12.99",
        monthlyLabel: "/ month",
        description: "100k issues, requests, task runs",
    },
    {
        id: "premium",
        name: "Premium",
        limit: "1mil",
        price: "$24.99",
        monthlyLabel: "/ month",
        description: "1mil issues, requests, task runs",
    },
    {
        id: "enterprise",
        name: "Enterprise",
        limit: "200mil",
        price: "$499.99",
        monthlyLabel: "/ month",
        description: "200mil issues, requests, task runs",
    },
    {
        id: "custom",
        name: "Custom",
        limit: "Unlimited",
        price: "Contact Us",
        description: "We are open to accommodating specific workloads and requirements.",
    },
];

export function PricingCalculator() {
    return (
        <div className="w-full max-w-4xl mx-auto">
            <div className="bg-white rounded-2xl border border-zinc-200 overflow-hidden">
                {/* Pricing Table */}
                <div>
                    <div className="grid grid-cols-12 gap-4 px-6 py-3 bg-zinc-50/50 text-xs font-semibold text-zinc-500 uppercase tracking-wider border-b border-zinc-100">
                        <div className="col-span-3">Tier</div>
                        <div className="col-span-3">Monthly Requests</div>
                        <div className="col-span-3">Base Price</div>
                        <div className="col-span-3">Includes</div>
                    </div>

                    <div className="divide-y divide-zinc-100">
                        {TIERS.map((tier) => (
                            <div
                                key={tier.id}
                                className="grid grid-cols-12 gap-4 px-6 py-5 items-center hover:bg-zinc-50 transition-colors"
                            >
                                <div className="col-span-3 font-bold text-zinc-900 flex items-center gap-3">
                                    {tier.name}
                                </div>
                                <div className="col-span-3 text-zinc-600 font-medium text-sm">
                                    {tier.limit}
                                </div>
                                <div className="col-span-3 text-zinc-900 font-bold">
                                    {tier.price}
                                    <span className="text-zinc-400 font-normal text-xs">{tier.monthlyLabel}</span>
                                </div>
                                <div className="col-span-3 text-zinc-500 text-xs text-balance leading-relaxed">
                                    {tier.description}
                                </div>
                            </div>
                        ))}
                    </div>
                </div>
            </div>

            <div className="mt-8 flex justify-center">
                <Link href="http://cloud.tracewayapp.com/register">
                    <Button size="lg" className="h-10 px-6 text-sm bg-[#4ba3f7] text-white hover:bg-[#3b93e7] font-bold shadow-sm shadow-blue-400/20">
                        Try for free <ArrowRight className="ml-2 h-4 w-4" />
                    </Button>
                </Link>
            </div>
        </div>
    );
}
