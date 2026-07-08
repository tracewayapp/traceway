import { cn } from "@/lib/utils";

type IsoDiagramProps = {
  src: string;
  alt: string;
  caption?: string;
  /** Draw a rounded dark panel around the diagram — use inside light bands. */
  framed?: boolean;
  /** Constrain the diagram width (default centered, full container width). */
  maxWidth?: number;
  priority?: boolean;
  className?: string;
};

// Isometric architecture diagram rendered by iso-topology (diagrams/scenes/*.yaml).
// The SVG paints its own #05070C canvas, so it sits seamlessly on --ink-0
// sections; pass `framed` to wrap it as a dark tile inside a light band.
export function IsoDiagram({
  src,
  alt,
  caption,
  framed = false,
  maxWidth,
  priority = false,
  className,
}: IsoDiagramProps) {
  return (
    <figure className={cn("mx-auto w-full", className)} style={maxWidth ? { maxWidth } : undefined}>
      {/* eslint-disable-next-line @next/next/no-img-element */}
      <img
        src={src}
        alt={alt}
        loading={priority ? "eager" : "lazy"}
        decoding="async"
        className={cn(
          "block h-auto w-full",
          framed &&
            "rounded-2xl border border-hair-2 bg-ink-0 shadow-[0_30px_80px_-40px_rgba(0,0,0,0.85)]",
        )}
      />
      {caption ? (
        <figcaption className="mt-4 text-center font-mono text-[11px] uppercase tracking-[0.14em] text-fg-3">
          {caption}
        </figcaption>
      ) : null}
    </figure>
  );
}
