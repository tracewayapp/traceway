import { MoveRight, MoveDown } from "lucide-react";

const RAW_LINES = [
  "n()",
  "",
  "app.min.js:1:63",
  "t()",
  "",
  "app.min.js:1:129",
  "",
  "app.min.js:1:146",
  "",
  "app.min.js:1:164",
];

const RESOLVED_FRAMES = [
  { fn: "validateUser", loc: "../src/user.ts:8:11", top: true },
  { fn: "handleSignup", loc: "../src/index.ts:4:10" },
  { fn: "<anonymous>", loc: "../src/index.ts:7:1" },
  { fn: "<anonymous>", loc: "../src/index.ts:7:29" },
];

const CARD_SHADOW = "0 18px 40px -24px rgba(10, 14, 24, 0.25)";

function CardLabel({ text }: { text: string }) {
  return (
    <p
      className="mb-3 text-center text-[11px] uppercase tracking-[0.14em]"
      style={{ fontFamily: "var(--font-mono)", color: "var(--fg-2)" }}
    >
      {text}
    </p>
  );
}

export function SymbolicationBeforeAfter() {
  return (
    <section className="wrap pt-16 pb-4">
      <div className="mx-auto grid max-w-5xl items-center gap-6 md:grid-cols-[1fr_auto_1fr] md:gap-5">
        <div className="min-w-0">
          <CardLabel text="What the browser sends" />
          <div
            className="overflow-hidden rounded-xl px-6 py-5 text-[12px] leading-[1.7]"
            style={{
              fontFamily: "var(--font-mono)",
              overflowWrap: "anywhere",
              background: "var(--ink-0)",
              border: "1px solid var(--hair-2)",
              boxShadow: CARD_SHADOW,
            }}
          >
            <div style={{ color: "var(--crit)" }}>Error: user has no name</div>
            <div className="mt-3" style={{ color: "var(--fg-3)" }}>
              {RAW_LINES.map((line, i) => (
                <div key={i} className="min-h-[1.2em]">
                  {line}
                </div>
              ))}
            </div>
          </div>
        </div>

        <div className="flex items-center justify-center py-1 md:py-0">
          <MoveRight
            className="hidden h-6 w-6 md:block"
            style={{ color: "var(--fg-3)" }}
            aria-hidden
          />
          <MoveDown
            className="h-6 w-6 md:hidden"
            style={{ color: "var(--fg-3)" }}
            aria-hidden
          />
        </div>

        <div className="min-w-0">
          <CardLabel text="What Traceway shows you" />
          <div
            className="overflow-hidden rounded-xl text-[12px] leading-[1.7]"
            style={{
              fontFamily: "var(--font-mono)",
              background: "var(--ink-0)",
              border: "1px solid var(--hair-2)",
              boxShadow: CARD_SHADOW,
            }}
          >
            <div
              className="px-5 py-3.5"
              style={{
                color: "var(--crit)",
                background: "color-mix(in oklab, var(--ink-2) 70%, transparent)",
                borderBottom: "1px solid var(--hair)",
              }}
            >
              Error: user has no name
            </div>
            {RESOLVED_FRAMES.map((f, i) => (
              <div
                key={i}
                className="flex flex-wrap items-baseline gap-x-3 px-5 py-3"
                style={{
                  borderTop: i > 0 ? "1px solid var(--hair)" : "none",
                  borderLeft: `3px solid ${f.top ? "var(--a2)" : "var(--ink-5)"}`,
                  overflowWrap: "anywhere",
                }}
              >
                <span style={{ color: "var(--fg-0)", fontWeight: 600 }}>{f.fn}</span>
                <span style={{ color: "var(--fg-3)" }}>{f.loc}</span>
              </div>
            ))}
          </div>
        </div>
      </div>
    </section>
  );
}
