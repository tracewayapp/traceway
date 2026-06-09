"use client";

import { useEffect } from "react";
import { usePathname } from "next/navigation";

type ScrollTriggerInstance = { kill: () => void };

export function MotionPolish() {
  const pathname = usePathname();

  useEffect(() => {
    if (typeof window === "undefined") return;
    if (window.matchMedia("(prefers-reduced-motion: reduce)").matches) return;
    // The blog is a reading experience, no entrance/scroll animations.
    if (pathname?.startsWith("/blog") || pathname?.startsWith("/releases"))
      return;

    let cancelled = false;
    const triggers: ScrollTriggerInstance[] = [];
    const cleanups: Array<() => void> = [];

    (async () => {
      const [{ default: gsap }, { ScrollTrigger }] = await Promise.all([
        import("gsap"),
        import("gsap/ScrollTrigger"),
      ]);

      if (cancelled) return;
      gsap.registerPlugin(ScrollTrigger);

      // Hero entrance: quick, small, once
      const hero = document.querySelector(".hero");
      if (hero) {
        const tl = gsap.timeline({ defaults: { ease: "power3.out" } });
        tl.from(".hero .chip", { y: 10, opacity: 0, duration: 0.45 }, 0)
          .from(".hero h1", { y: 14, opacity: 0, duration: 0.55 }, 0.08)
          .from(".hero-sub", { y: 10, opacity: 0, duration: 0.45 }, 0.18);
      }

      // Number counters
      document.querySelectorAll<HTMLElement>(".stats-strip .num").forEach((el) => {
        const raw = (el.textContent ?? "").trim();
        const match = raw.match(/([\d.]+)/);
        if (!match) return;
        const target = parseFloat(match[1]);
        const prefix = raw.slice(0, match.index);
        const suffix = raw.slice((match.index ?? 0) + match[0].length);
        const isFloat = match[1].includes(".");
        const hasEm = !!el.querySelector("em");
        const obj = { v: 0 };
        const t = ScrollTrigger.create({
          trigger: el,
          start: "top 88%",
          once: true,
          onEnter: () => {
            gsap.to(obj, {
              v: target,
              duration: 1.6,
              ease: "power2.out",
              onUpdate: () => {
                const n = isFloat ? obj.v.toFixed(1) : Math.round(obj.v);
                el.innerHTML = hasEm ? `${prefix}<em>${n}${suffix}</em>` : `${prefix}${n}${suffix}`;
              },
            });
          },
        });
        triggers.push(t);
      });

      // Cost-bar fills
      document.querySelectorAll<HTMLElement>("[data-cost-mount]").forEach((mount) => {
        const t = ScrollTrigger.create({
          trigger: mount,
          start: "top 80%",
          once: true,
          onEnter: () => {
            requestAnimationFrame(() => {
              const fills = mount.querySelectorAll<HTMLElement>(".cost-bar .fill");
              fills.forEach((f) => {
                const target = f.getAttribute("data-w");
                if (!target) return;
                gsap.fromTo(
                  f,
                  { width: 0 },
                  { width: `${target}%`, duration: 1.2, ease: "power3.out" }
                );
              });
            });
          },
        });
        triggers.push(t);
      });

      // Terminal type-on
      document.querySelectorAll<HTMLElement>(".term").forEach((term) => {
        const lines = term.querySelectorAll(".term-line");
        if (!lines.length) return;
        gsap.set(lines, { opacity: 0, x: -8 });
        const t = ScrollTrigger.create({
          trigger: term,
          start: "top 80%",
          once: true,
          onEnter: () => {
            gsap.to(lines, {
              opacity: 1,
              x: 0,
              duration: 0.45,
              ease: "power2.out",
              stagger: 0.15,
            });
          },
        });
        triggers.push(t);
      });

      // Nav thickening on scroll
      const nav = document.querySelector<HTMLElement>(".site-nav");
      if (nav) {
        const t = ScrollTrigger.create({
          start: 60,
          end: 99999,
          onUpdate: (self) => {
            nav.dataset.scrolled = self.progress > 0 ? "true" : "false";
          },
        });
        triggers.push(t);
      }

      // Refresh after layout settles
      const r1 = setTimeout(() => ScrollTrigger.refresh(), 600);
      const r2 = setTimeout(() => ScrollTrigger.refresh(), 1500);
      cleanups.push(() => {
        clearTimeout(r1);
        clearTimeout(r2);
      });
    })();

    return () => {
      cancelled = true;
      triggers.forEach((t) => t.kill());
      cleanups.forEach((c) => c());
    };
  }, [pathname]);

  return null;
}
