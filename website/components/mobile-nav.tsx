"use client";

import { useState, useEffect } from "react";
import Link from "next/link";
import { createPortal } from "react-dom";
import { Button } from "@/components/ui/button";
import { Github, Menu, X } from "lucide-react";

export function MobileNav() {
    const [isOpen, setIsOpen] = useState(false);
    const [mounted, setMounted] = useState(false);

    useEffect(() => {
        setMounted(true);
    }, []);

    useEffect(() => {
        if (isOpen) {
            document.body.style.overflow = "hidden";
        } else {
            document.body.style.overflow = "auto";
        }
        return () => {
            document.body.style.overflow = "auto";
        }
    }, [isOpen]);

    return (
        <div className="md:hidden">
            <Button
                variant="ghost"
                size="icon"
                className="text-zinc-600"
                onClick={() => setIsOpen(!isOpen)}
            >
                {isOpen ? (
                    <X className="h-6 w-6" />
                ) : (
                    <Menu className="h-6 w-6" />
                )}
                <span className="sr-only">Toggle menu</span>
            </Button>

            {isOpen && mounted && createPortal(
                <div className="md:hidden fixed top-14 left-0 right-0 bottom-0 bg-white z-50 p-4 overflow-y-auto border-t border-zinc-100 animate-in slide-in-from-top-2 fade-in duration-200 flex flex-col">
                    <div className="flex flex-col gap-6 mt-4">
                        <div className="text-xs font-semibold text-zinc-400 uppercase tracking-wider">Product</div>
                        <Link
                            href="/product/issue-tracking"
                            className="text-lg font-medium text-zinc-600 hover:text-zinc-900 transition-colors pl-2"
                            onClick={() => setIsOpen(false)}
                        >
                            Issue Tracking
                        </Link>
                        <Link
                            href="/product/performance"
                            className="text-lg font-medium text-zinc-600 hover:text-zinc-900 transition-colors pl-2"
                            onClick={() => setIsOpen(false)}
                        >
                            Performance
                        </Link>
                        <Link
                            href="/product/session-replay"
                            className="text-lg font-medium text-zinc-600 hover:text-zinc-900 transition-colors pl-2"
                            onClick={() => setIsOpen(false)}
                        >
                            Session Replay
                        </Link>
                        <div className="border-t border-zinc-100"></div>
                        <Link
                            href="/cloud"
                            className="text-lg font-medium text-zinc-600 hover:text-zinc-900 transition-colors"
                            onClick={() => setIsOpen(false)}
                        >
                            Cloud
                        </Link>
                        <Link
                            href="https://docs.tracewayapp.com"
                            className="text-lg font-medium text-zinc-600 hover:text-zinc-900 transition-colors"
                            onClick={() => setIsOpen(false)}
                        >
                            Docs
                        </Link>
                        <Link
                            href="https://github.com/tracewayapp/traceway"
                            target="_blank"
                            rel="noopener noreferrer"
                            className="text-lg font-medium text-zinc-600 hover:text-zinc-900 transition-colors flex items-center gap-2"
                            onClick={() => setIsOpen(false)}
                        >
                            GitHub <Github className="h-4 w-4" />
                        </Link>
                    </div>
                    <div className="flex-1"></div>
                    <div className="mt-8 flex flex-col gap-4">
                        <Link href="http://cloud.tracewayapp.com/register" className="w-full" onClick={() => setIsOpen(false)}>
                            <Button className="w-full bg-[#4ba3f7] text-white hover:bg-[#3b93e7] font-medium h-12 text-lg">
                                Try for free
                            </Button>
                        </Link>
                        <Link href="http://cloud.tracewayapp.com/login" className="w-full" onClick={() => setIsOpen(false)}>
                            <Button variant="outline" className="w-full h-12 text-lg text-zinc-600 hover:text-zinc-900 hover:bg-zinc-100">
                                Sign in
                            </Button>
                        </Link>
                    </div>
                </div>,
                document.body
            )}
        </div>
    );
}
