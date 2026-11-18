"use client";

import { useEffect, useRef, useState } from "react";
import { ChevronDown } from "lucide-react";
import type { BlogHeading } from "@/lib/blog-headings";

// A heading counts as reached once it is this close to the top of the
// viewport: the sticky site header plus a little air.
const ACTIVE_OFFSET = 120;

function useActiveHeading(headings: BlogHeading[]): string | null {
  const [active, setActive] = useState<string | null>(null);

  useEffect(() => {
    let frame = 0;
    const update = () => {
      frame = 0;
      let current: string | null = null;
      for (const heading of headings) {
        const el = document.getElementById(heading.id);
        if (!el) continue;
        if (el.getBoundingClientRect().top - ACTIVE_OFFSET <= 0) current = heading.id;
        else break;
      }
      const atEnd =
        window.innerHeight + window.scrollY >=
        document.documentElement.scrollHeight - 4;
      if (atEnd && headings.length > 0) {
        const last = document.getElementById(headings[headings.length - 1].id);
        if (last && last.getBoundingClientRect().top < window.innerHeight) {
          current = headings[headings.length - 1].id;
        }
      }
      setActive(current);
    };
    const onScroll = () => {
      if (!frame) frame = requestAnimationFrame(update);
    };
    update();
    window.addEventListener("scroll", onScroll, { passive: true });
    window.addEventListener("resize", onScroll);
    return () => {
      window.removeEventListener("scroll", onScroll);
      window.removeEventListener("resize", onScroll);
      if (frame) cancelAnimationFrame(frame);
    };
  }, [headings]);

  return active;
}

function TocList({
  headings,
  active,
  onNavigate,
}: {
  headings: BlogHeading[];
  active: string | null;
  onNavigate?: () => void;
}) {
  return (
    <ul className="blog-toc-list">
      {headings.map((heading) => (
        <li key={heading.id}>
          <a
            href={`#${heading.id}`}
            data-depth={heading.depth}
            data-toc-id={heading.id}
            aria-current={heading.id === active ? "location" : undefined}
            className="blog-toc-link"
            onClick={onNavigate}
          >
            {heading.text}
          </a>
        </li>
      ))}
    </ul>
  );
}

export function BlogToc({ headings }: { headings: BlogHeading[] }) {
  const active = useActiveHeading(headings);
  const railRef = useRef<HTMLElement>(null);
  const detailsRef = useRef<HTMLDetailsElement>(null);

  useEffect(() => {
    const rail = railRef.current;
    if (!rail || !active) return;
    const link = rail.querySelector<HTMLElement>(
      `[data-toc-id="${CSS.escape(active)}"]`,
    );
    if (!link) return;
    const top = link.offsetTop;
    const bottom = top + link.offsetHeight;
    if (top < rail.scrollTop + 24) rail.scrollTo({ top: top - 24 });
    else if (bottom > rail.scrollTop + rail.clientHeight - 24)
      rail.scrollTo({ top: bottom - rail.clientHeight + 24 });
  }, [active]);

  if (headings.length === 0) return null;

  const current = headings.find((h) => h.id === active);

  return (
    <>
      <nav ref={railRef} className="blog-toc-rail" aria-label="On this page">
        <p className="blog-toc-title">On this page</p>
        <TocList headings={headings} active={active} />
      </nav>

      <details ref={detailsRef} className="blog-toc-inline">
        <summary>
          <span className="blog-toc-inline-label">
            <span className="blog-toc-title">On this page</span>
            {current && (
              <span className="blog-toc-inline-current">{current.text}</span>
            )}
          </span>
          <ChevronDown className="blog-toc-chevron" aria-hidden="true" />
        </summary>
        <TocList
          headings={headings}
          active={active}
          onNavigate={() => {
            if (detailsRef.current) detailsRef.current.open = false;
          }}
        />
      </details>
    </>
  );
}
