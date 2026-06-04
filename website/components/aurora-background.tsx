import { cn } from "@/lib/utils";

type Variant = "default" | "hero" | "cta";

export function AuroraBackground({
  variant = "default",
  className,
}: {
  variant?: Variant;
  className?: string;
}) {
  const intensity =
    variant === "hero" ? "hero" : variant === "cta" ? "cta" : "default";
  return (
    <div
      aria-hidden
      data-aurora={intensity}
      className={cn(
        "pointer-events-none absolute inset-0 -z-10 overflow-hidden",
        className
      )}
    >
      <div
        className="absolute -inset-24"
        style={{
          background:
            variant === "hero"
              ? `radial-gradient(900px 500px at 80% 0%, color-mix(in oklab, var(--a1) 6%, transparent), transparent 60%)`
              : variant === "cta"
              ? `radial-gradient(700px 400px at 50% 0%, color-mix(in oklab, var(--a1) 5%, transparent), transparent 65%)`
              : `radial-gradient(600px 380px at 80% 0%, color-mix(in oklab, var(--a1) 4%, transparent), transparent 65%)`,
        }}
      />
    </div>
  );
}
