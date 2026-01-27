
import Link from "next/link";
import Image from "next/image";
import { Button } from "@/components/ui/button";
import { Github } from "lucide-react";

export function SiteHeader() {
    return (
        <nav className="border-b border-zinc-100 bg-white/80 backdrop-blur-md sticky top-0 z-50">
            <div className="container mx-auto px-4 h-14 flex items-center justify-between">
                <div className="flex items-center gap-8">
                    <Link href="/" className="flex items-center gap-2">
                        <Image
                            src="/images/logo.png"
                            alt="Traceway Logo"
                            width={100}
                            height={100}
                            className="w-auto h-8"
                        />
                    </Link>
                    <div className="hidden md:flex items-center gap-6">
                        <Link href="/cloud" className="text-sm font-medium text-zinc-600 hover:text-zinc-900 transition-colors">
                            Cloud
                        </Link>
                        <Link href="https://docs.tracewayapp.com" className="text-sm font-medium text-zinc-600 hover:text-zinc-900 transition-colors">
                            Docs
                        </Link>
                    </div>
                </div>
                <div className="flex items-center gap-4">
                    <Link href="https://github.com/tracewayapp/traceway" target="_blank" rel="noopener noreferrer" className="hidden sm:block">
                        <Button variant="ghost" size="icon" className="h-8 w-8 text-zinc-600 hover:text-zinc-900 hover:bg-zinc-100">
                            <Github className="h-4 w-4" />
                            <span className="sr-only">GitHub</span>
                        </Button>
                    </Link>
                    <div className="flex items-center gap-2">
                        <Link href="http://cloud.tracewayapp.com/login">
                            <Button variant="ghost" className="text-zinc-600 hover:text-zinc-900 hover:bg-zinc-100">
                                Sign in
                            </Button>
                        </Link>
                        <Link href="http://cloud.tracewayapp.com/register">
                            <Button className="bg-[#4ba3f7] text-white hover:bg-[#3b93e7] font-medium">
                                Try for free
                            </Button>
                        </Link>
                    </div>
                </div>
            </div>
        </nav>
    );
}
