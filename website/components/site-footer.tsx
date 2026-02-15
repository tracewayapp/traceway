import Link from "next/link";

export function SiteFooter() {
    return (
        <footer className="py-10 border-t border-zinc-200 bg-white">
            <div className="container mx-auto px-4">
                <div className="grid grid-cols-2 md:grid-cols-4 gap-8 text-sm">
                    <div>
                        <h4 className="font-semibold text-zinc-900 mb-3">Product</h4>
                        <ul className="space-y-2 text-zinc-500">
                            <li>
                                <Link href="/product/issue-tracking" className="hover:text-zinc-900 transition-colors">
                                    Issue Tracking
                                </Link>
                            </li>
                            <li>
                                <Link href="/product/performance" className="hover:text-zinc-900 transition-colors">
                                    Performance
                                </Link>
                            </li>
                            <li>
                                <Link href="/product/session-replay" className="hover:text-zinc-900 transition-colors">
                                    Session Replay
                                </Link>
                            </li>
                        </ul>
                    </div>
                    <div>
                        <h4 className="font-semibold text-zinc-900 mb-3">Resources</h4>
                        <ul className="space-y-2 text-zinc-500">
                            <li>
                                <Link href="https://docs.tracewayapp.com" className="hover:text-zinc-900 transition-colors">
                                    Docs
                                </Link>
                            </li>
                            <li>
                                <Link href="https://github.com/tracewayapp/traceway" target="_blank" className="hover:text-zinc-900 transition-colors">
                                    GitHub
                                </Link>
                            </li>
                            <li>
                                <Link href="https://cloud.tracewayapp.com/login?email=demo@tracewayapp.com&password=demoaccount!" className="hover:text-zinc-900 transition-colors">
                                    Live Demo
                                </Link>
                            </li>
                        </ul>
                    </div>
                    <div>
                        <h4 className="font-semibold text-zinc-900 mb-3">Hosting</h4>
                        <ul className="space-y-2 text-zinc-500">
                            <li>
                                <Link href="/cloud" className="hover:text-zinc-900 transition-colors">
                                    Cloud
                                </Link>
                            </li>
                        </ul>
                    </div>
                    <div>
                        <h4 className="font-semibold text-zinc-900 mb-3">About</h4>
                        <ul className="space-y-2 text-zinc-500">
                            <li>
                                <Link href="/privacy-policy" className="hover:text-zinc-900 transition-colors">
                                    Privacy Policy
                                </Link>
                            </li>
                            <li>
                                <Link href="/terms-of-use" className="hover:text-zinc-900 transition-colors">
                                    Terms of Use
                                </Link>
                            </li>
                            <li>
                                <Link href="/contact" className="hover:text-zinc-900 transition-colors">
                                    Contact Us
                                </Link>
                            </li>
                        </ul>
                    </div>
                </div>
                <div className="mt-8 pt-6 border-t border-zinc-100 text-xs text-zinc-400">
                    &copy; {new Date().getFullYear()} Traceway. All rights reserved.
                </div>
            </div>
        </footer>
    );
}
