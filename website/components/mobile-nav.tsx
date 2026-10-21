"use client";

import { useState } from "react";
import Link from "next/link";
import { Menu } from "lucide-react";
import { GITHUB_URL, DISCORD_URL } from "@/lib/links";
import {
  Sheet,
  SheetContent,
  SheetTrigger,
  SheetTitle,
  SheetDescription,
} from "@/components/ui/sheet";

type NavItem = {
  title: string;
  href: string;
};

const SECONDARY: (NavItem & { external?: boolean })[] = [
  { title: "Cloud", href: "/cloud" },
  { title: "Blog", href: "/blog" },
  { title: "Releases", href: "/releases" },
  { title: "Benchmarks", href: "/benchmarks" },
  { title: "Docs", href: "https://docs.tracewayapp.com", external: true },
  { title: "GitHub", href: GITHUB_URL, external: true },
  { title: "Discord", href: DISCORD_URL, external: true },
];

export function MobileNav({
  pillars,
  specialized,
}: {
  pillars: NavItem[];
  specialized: NavItem[];
}) {
  const [open, setOpen] = useState(false);

  return (
    <div className="md:hidden">
      <Sheet open={open} onOpenChange={setOpen}>
        <SheetTrigger asChild>
          <button
            className="inline-flex items-center justify-center h-9 w-9 rounded-md text-[color:var(--fg-1)] hover:text-[color:var(--fg-0)] hover:bg-[color:var(--ink-2)] transition-colors"
            aria-label="Open menu"
          >
            <Menu className="h-5 w-5" />
          </button>
        </SheetTrigger>
        <SheetContent
          side="right"
          className="w-full sm:max-w-[360px] border-l-[color:var(--hair)] bg-[color:var(--ink-0)] p-0 flex flex-col"
        >
          <SheetTitle className="sr-only">Menu</SheetTitle>
          <SheetDescription className="sr-only">Traceway product navigation</SheetDescription>

          <div className="px-6 pt-16 pb-6 overflow-y-auto flex-1">
            <NavSection
              eyebrow="Platform"
              items={pillars}
              onNavigate={() => setOpen(false)}
            />
            <div className="mt-8">
              <NavSection
                eyebrow="Specialized"
                items={specialized}
                onNavigate={() => setOpen(false)}
              />
            </div>

            <div
              className="mt-8 pt-6 flex flex-col"
              style={{ borderTop: "1px solid var(--hair)" }}
            >
              {SECONDARY.map((item) => (
                <MobileLink
                  key={item.href}
                  item={item}
                  external={item.external}
                  onClick={() => setOpen(false)}
                />
              ))}
            </div>
          </div>

          <div
            className="p-6 flex flex-col gap-3"
            style={{ borderTop: "1px solid var(--hair)" }}
          >
            <Link
              href="https://cloud.tracewayapp.com/register"
              onClick={() => setOpen(false)}
              className="btn btn-accent w-full justify-center"
            >
              Start for free
            </Link>
            <Link
              href="https://cloud.tracewayapp.com/login"
              onClick={() => setOpen(false)}
              className="btn btn-ghost w-full justify-center"
            >
              Sign in
            </Link>
          </div>
        </SheetContent>
      </Sheet>
    </div>
  );
}

function NavSection({
  eyebrow,
  items,
  onNavigate,
}: {
  eyebrow: string;
  items: NavItem[];
  onNavigate?: () => void;
}) {
  return (
    <div>
      <div
        className="text-[13.5px] font-medium uppercase tracking-[0.16em]"
        style={{ fontFamily: "var(--font-mono)", color: "var(--fg-3)" }}
      >
        {eyebrow}
      </div>
      <div className="mt-3 flex flex-col">
        {items.map((item) => (
          <MobileLink key={item.href} item={item} onClick={onNavigate} />
        ))}
      </div>
    </div>
  );
}

function MobileLink({
  item,
  external,
  onClick,
}: {
  item: NavItem;
  external?: boolean;
  onClick?: () => void;
}) {
  return (
    <Link
      href={item.href}
      onClick={onClick}
      {...(external ? { target: "_blank", rel: "noopener noreferrer" } : {})}
      className="-mx-2 block rounded-md px-2 py-[9px] text-[15px] font-medium transition-colors text-[color:var(--fg-1)] hover:text-[color:var(--fg-0)] hover:bg-[color:rgba(255,255,255,0.04)]"
      style={{ fontFamily: "var(--font-body)", letterSpacing: "-0.01em" }}
    >
      {item.title}
    </Link>
  );
}
