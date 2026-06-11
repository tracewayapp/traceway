import { Terminal } from "@/components/terminal";

export function AgentDebugTerminal({ className }: { className?: string }) {
  return (
    <Terminal
      className={className}
      title="claude · ~/shop-api"
      lines={[
        {
          ln: "1",
          type: "tx",
          content: (
            <>
              <span className="cmd">❯</span> /traceway users report 500s on
              checkout since this morning
            </>
          ),
        },
        { ln: "2", type: "mute", content: "# querying production telemetry…" },
        {
          ln: "3",
          type: "tx",
          content: (
            <>
              <span className="cmd">$</span> traceway exceptions list --since
              24h --search checkout
            </>
          ),
        },
        {
          ln: "4",
          type: "tx",
          content: "TypeError: cart is null · 412 events · first seen 09:14",
        },
        {
          ln: "5",
          type: "tx",
          content: (
            <>
              <span className="cmd">$</span> traceway logs query --trace-id
              9f2c41d8
            </>
          ),
        },
        {
          ln: "6",
          type: "mute",
          content: "# reading src/checkout/session.ts:42",
        },
        {
          ln: "7",
          type: "ok",
          content: "# ✓ root cause: repeat purchase reuses an expired cart",
        },
        {
          ln: "8",
          type: "ok",
          content: "# ✓ fix ready for review in src/checkout/session.ts",
        },
      ]}
      showCursor
    />
  );
}
