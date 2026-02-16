"use client";

import { useState, useRef, useEffect } from "react";
import Link from "next/link";
import Image from "next/image";

import { Github, AlertCircle, Activity, Video, ChevronDown } from "lucide-react";
import { MobileNav } from "@/components/mobile-nav";

export function SiteHeader() {
    const [open, setOpen] = useState(false);
    const timeoutRef = useRef<NodeJS.Timeout | null>(null);
    const dropdownRef = useRef<HTMLDivElement>(null);

    function handleEnter() {
        if (timeoutRef.current) clearTimeout(timeoutRef.current);
        setOpen(true);
    }

    function handleLeave() {
        timeoutRef.current = setTimeout(() => setOpen(false), 150);
    }

    useEffect(() => {
        return () => {
            if (timeoutRef.current) clearTimeout(timeoutRef.current);
        };
    }, []);

    return (
        <nav className="border-b border-zinc-100 bg-white/80 backdrop-blur-md sticky top-0 z-50">
            <div className="container mx-auto px-4 h-14 flex items-center justify-between">
                <div className="flex items-center gap-6">
                    <Link href="/" className="flex items-center gap-2">
                        <Image
                            src="/images/logo.png"
                            alt="Traceway Logo"
                            width={100}
                            height={100}
                            className="w-auto h-8"
                        />
                    </Link>
                    <div className="hidden md:flex items-center gap-1">
                        <div
                            ref={dropdownRef}
                            className="relative"
                            onMouseEnter={handleEnter}
                            onMouseLeave={handleLeave}
                        >
                            <button className="inline-flex items-center gap-1 text-sm font-medium text-zinc-600 hover:text-zinc-900 transition-colors h-9 px-3 rounded-md hover:bg-zinc-100">
                                Product
                                <ChevronDown className={`size-3 transition-transform duration-100 ${open ? "rotate-180" : ""}`} />
                            </button>
                            {open && (
                                <div className="absolute top-full left-0 mt-1 w-[340px] rounded-md border border-zinc-200 bg-white shadow-lg p-2 animate-in fade-in zoom-in-95 duration-75">
                                    <Link
                                        href="/product/issue-tracking"
                                        className="flex items-start gap-3 rounded-md p-3 hover:bg-zinc-50 transition-colors"
                                        onClick={() => setOpen(false)}
                                    >
                                        <div className="w-8 h-8 bg-red-50 rounded-lg flex items-center justify-center shrink-0 mt-0.5">
                                            <AlertCircle className="w-4 h-4 text-red-600" />
                                        </div>
                                        <div>
                                            <div className="text-sm font-medium text-zinc-900">Issue Tracking</div>
                                            <p className="text-xs text-zinc-500 mt-0.5">Automatic ranking, error grouping, and regression detection</p>
                                        </div>
                                    </Link>
                                    <Link
                                        href="/product/performance"
                                        className="flex items-start gap-3 rounded-md p-3 hover:bg-zinc-50 transition-colors"
                                        onClick={() => setOpen(false)}
                                    >
                                        <div className="w-8 h-8 bg-blue-50 rounded-lg flex items-center justify-center shrink-0 mt-0.5">
                                            <Activity className="w-4 h-4 text-blue-600" />
                                        </div>
                                        <div>
                                            <div className="text-sm font-medium text-zinc-900">Performance</div>
                                            <p className="text-xs text-zinc-500 mt-0.5">P50/P95/P99 percentiles, waterfall traces, and server metrics</p>
                                        </div>
                                    </Link>
                                    <Link
                                        href="/product/session-replay"
                                        className="flex items-start gap-3 rounded-md p-3 hover:bg-zinc-50 transition-colors"
                                        onClick={() => setOpen(false)}
                                    >
                                        <div className="w-8 h-8 bg-purple-50 rounded-lg flex items-center justify-center shrink-0 mt-0.5">
                                            <Video className="w-4 h-4 text-purple-600" />
                                        </div>
                                        <div>
                                            <div className="text-sm font-medium text-zinc-900">Session Replay</div>
                                            <p className="text-xs text-zinc-500 mt-0.5">See exactly what users did before every exception</p>
                                        </div>
                                    </Link>
                                </div>
                            )}
                        </div>
                        <Link href="/cloud" className="text-sm font-medium text-zinc-600 hover:text-zinc-900 transition-colors h-9 px-3 flex items-center rounded-md hover:bg-zinc-100">
                            Cloud
                        </Link>
                        <Link href="https://docs.tracewayapp.com" className="text-sm font-medium text-zinc-600 hover:text-zinc-900 transition-colors h-9 px-3 flex items-center rounded-md hover:bg-zinc-100">
                            Docs
                        </Link>
                    </div>
                </div>

                {/* Desktop Actions */}
                <div className="hidden md:flex items-center gap-4">
                    <Link href="https://github.com/tracewayapp/traceway" target="_blank" rel="noopener noreferrer" className="inline-flex items-center justify-center gap-2 rounded-md text-sm font-medium transition-all cursor-pointer h-8 w-8 text-zinc-600 hover:text-zinc-900 hover:bg-zinc-100">
                            <Github className="h-4 w-4" />
                            <span className="sr-only">GitHub</span>
                    </Link>
                    <div className="flex items-center gap-2">
                        <Link href="http://cloud.tracewayapp.com/login" className="inline-flex items-center justify-center gap-2 rounded-md text-sm font-medium transition-all cursor-pointer h-9 px-4 text-zinc-600 hover:text-zinc-900 hover:bg-zinc-100">
                                Sign in
                        </Link>
                        <Link href="http://cloud.tracewayapp.com/register" className="inline-flex items-center justify-center gap-2 rounded-md text-sm font-medium transition-all cursor-pointer h-9 px-4 bg-[#4ba3f7] text-white hover:bg-[#3b93e7]">
                                Try for free
                        </Link>
                    </div>
                </div>

                {/* Mobile Menu */}
                <MobileNav />
            </div>
        </nav>
    );
}
