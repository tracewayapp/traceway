"use client";

import { useState } from "react";
import { ListFilter, Network, ScanSearch } from "lucide-react";
import { ScreenshotZoom } from "@/components/screenshot-zoom";
import { tracingScreenshots } from "@/lib/tracing-screenshots";

const icons = [ListFilter, Network, ScanSearch];

export function TracingShowcase() {
  const [active, setActive] = useState(0);
  const view = tracingScreenshots[active];

  return (
    <div className="overflow-hidden rounded-xl border border-hair-2 bg-ink-1">
      <div className="grid grid-cols-3 border-b border-hair-2" role="group" aria-label="Tracing screenshots">
        {tracingScreenshots.slice(0, 3).map(({ label }, index) => {
          const Icon = icons[index];
          return (
            <button
              key={label}
              type="button"
              aria-pressed={active === index}
              aria-controls="tracing-screenshot"
              onClick={() => setActive(index)}
              className={`flex cursor-pointer flex-col items-center justify-center gap-2 border-b-2 px-2 py-4 text-xs font-medium transition-colors focus-visible:outline-2 focus-visible:outline-offset-[-4px] focus-visible:outline-a2 sm:flex-row sm:gap-3 sm:px-5 sm:text-sm ${active === index ? "border-a2 bg-ink-3 text-fg-0" : "border-transparent text-fg-2 hover:bg-ink-2 hover:text-fg-0"}`}
            >
              <Icon className="h-4 w-4 shrink-0" aria-hidden="true" />
              {label}
            </button>
          );
        })}
      </div>
      <figure id="tracing-screenshot">
        <ScreenshotZoom images={tracingScreenshots} index={active} eager={active === 0} />
        <figcaption className="border-t border-hair-2 px-5 py-5 sm:px-7" aria-live="polite" aria-atomic="true">
          <p className="font-medium text-fg-0">{view.title}</p>
          <p className="mt-2 max-w-3xl text-sm leading-relaxed text-fg-2">{view.description}</p>
          <p className="mt-4 font-mono text-[11px] text-fg-2">Captured in Traceway · checkout demo data</p>
        </figcaption>
      </figure>
    </div>
  );
}
