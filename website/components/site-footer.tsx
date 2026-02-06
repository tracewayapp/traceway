import Link from "next/link";

export function SiteFooter() {
    return (
        <footer className="py-8 border-t border-zinc-200 bg-white">
            <div className="container mx-auto px-4 flex flex-col md:flex-row items-center justify-between text-zinc-500 text-xs">
                <div className="font-medium">
                    &copy; {new Date().getFullYear()} Traceway. All rights reserved.
                </div>
                <div className="flex items-center gap-6 mt-3 md:mt-0 font-medium">
                    <Link href="/product/issue-tracking" className="hover:text-zinc-900 transition-colors">
                        Issue Tracking
                    </Link>
                    <Link href="/product/performance" className="hover:text-zinc-900 transition-colors">
                        Performance
                    </Link>
                    <Link href="/cloud" className="hover:text-zinc-900 transition-colors">
                        Cloud
                    </Link>
                    <Link href="https://docs.tracewayapp.com" className="hover:text-zinc-900 transition-colors">
                        Docs
                    </Link>
                    <Link href="https://github.com/tracewayapp/traceway" className="hover:text-zinc-900 transition-colors">
                        GitHub
                    </Link>
                </div>
            </div>
        </footer>
    );
}
