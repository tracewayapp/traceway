"use client";

import { useState } from "react";
import Link from "next/link";
import { Github, Menu } from "lucide-react";
import { DiscordIcon } from "@/components/discord-icon";
import { GITHUB_URL, DISCORD_URL } from "@/lib/links";
import type { LucideIcon } from "lucide-react";
import {
  Sheet,
  SheetContent,
  SheetTrigger,
  SheetTitle,
  SheetDescription,
} from "@/components/ui/sheet";
import { Eyebrow } from "@/components/eyebrow";
import { cn } from "@/lib/utils";

type NavItem = {
  title: string;
  description: string;
  href: string;
  icon: LucideIcon;
};

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
            <Eyebrow className="block mb-3">Observability pillars</Eyebrow>
            <div className="flex flex-col">
              {pillars.map((p) => (
                <MobileLink key={p.href} item={p} onClick={() => setOpen(false)} />
              ))}
            </div>

            <Eyebrow className="block mt-8 mb-3">Specialized</Eyebrow>
            <div className="flex flex-col">
              {specialized.map((p) => (
                <MobileLink key={p.href} item={p} onClick={() => setOpen(false)} />
              ))}
            </div>

            <div
              className="mt-8 pt-6 flex flex-col gap-1"
              style={{ borderTop: "1px solid var(--hair)" }}
            >
              <Link
                href="/cloud"
                onClick={() => setOpen(false)}
                className="py-3 text-[16px] font-medium text-[color:var(--fg-0)] hover:text-[color:var(--a2)]"
                style={{ fontFamily: "var(--font-display)" }}
              >
                Cloud
              </Link>
              <Link
                href="/blog"
                onClick={() => setOpen(false)}
                className="py-3 text-[16px] font-medium text-[color:var(--fg-0)] hover:text-[color:var(--a2)]"
                style={{ fontFamily: "var(--font-display)" }}
              >
                Blog
              </Link>
              <Link
                href="https://docs.tracewayapp.com"
                target="_blank"
                rel="noopener noreferrer"
                onClick={() => setOpen(false)}
                className="py-3 text-[16px] font-medium text-[color:var(--fg-0)] hover:text-[color:var(--a2)]"
                style={{ fontFamily: "var(--font-display)" }}
              >
                Docs
              </Link>
              <Link
                href={GITHUB_URL}
                target="_blank"
                rel="noopener noreferrer"
                onClick={() => setOpen(false)}
                className="py-3 text-[16px] font-medium text-[color:var(--fg-0)] hover:text-[color:var(--a2)] inline-flex items-center gap-2"
                style={{ fontFamily: "var(--font-display)" }}
              >
                GitHub
                <Github className="h-4 w-4" />
              </Link>
              <Link
                href={DISCORD_URL}
                target="_blank"
                rel="noopener noreferrer"
                onClick={() => setOpen(false)}
                className="py-3 text-[16px] font-medium text-[color:var(--fg-0)] hover:text-[color:var(--a2)] inline-flex items-center gap-2"
                style={{ fontFamily: "var(--font-display)" }}
              >
                Discord
                <DiscordIcon className="h-4 w-4" />
              </Link>
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

function MobileLink({ item, onClick }: { item: NavItem; onClick?: () => void }) {
  const Icon = item.icon;
  return (
    <Link
      href={item.href}
      onClick={onClick}
      className={cn(
        "grid grid-cols-[32px_1fr] gap-3 items-start py-3 -mx-2 px-2 rounded-md transition-colors hover:bg-[color:var(--ink-2)]"
      )}
    >
      <div
        className="h-8 w-8 rounded-md grid place-items-center"
        style={{
          background: "rgba(255,255,255,0.04)",
          border: "1px solid var(--hair)",
          color: "var(--a2)",
        }}
      >
        <Icon className="h-4 w-4" />
      </div>
      <div>
        <div
          className="text-[15px] font-medium leading-tight"
          style={{ fontFamily: "var(--font-display)", color: "var(--fg-0)" }}
        >
          {item.title}
        </div>
        <div className="text-[12px] mt-0.5 leading-snug" style={{ color: "var(--fg-3)" }}>
          {item.description}
        </div>
      </div>
    </Link>
  );
}
