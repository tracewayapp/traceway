import Image from "next/image";
import { cn } from "@/lib/utils";

export function AgentDebugTerminal({ className }: { className?: string }) {
  return (
    <div className={cn("term", className)}>
      <div className="term-body">
        <div className="flex items-center gap-5">
          <Image
            src="/images/claudebot.png"
            alt="Claude"
            width={152}
            height={97}
            className="h-11 w-auto"
            style={{ imageRendering: "pixelated" }}
          />
          <div>
            <div>
              <span className="font-semibold text-fg-0">Claude Code</span>{" "}
              <span className="text-fg-3">v2.1.173</span>
            </div>
            <div className="text-fg-3">Fable 5 · Claude Max</div>
            <div className="text-fg-3">~/shop-api</div>
          </div>
        </div>

        <div className="mt-1 grid grid-cols-[14px_1fr] gap-x-2">
          <span className="text-fg-3">❯</span>
          <span className="whitespace-pre-wrap text-fg-0">
            /traceway users report 500s on checkout since this morning
          </span>
        </div>

        <div className="mt-1 grid grid-cols-[14px_1fr] gap-x-2">
          <span className="text-ok">⏺</span>
          <span className="whitespace-pre-wrap">
            <span className="text-fg-0">Bash</span>
            <span className="text-fg-2">
              (traceway exceptions list --since 24h --search checkout)
            </span>
          </span>
        </div>
        <div className="grid grid-cols-[14px_1fr] gap-x-2">
          <span />
          <span className="whitespace-pre-wrap text-fg-3">
            {"⎿  TypeError: cart is null · 412 events · first seen 09:14"}
          </span>
        </div>

        <div className="mt-1 grid grid-cols-[14px_1fr] gap-x-2">
          <span className="text-ok">⏺</span>
          <span className="whitespace-pre-wrap">
            <span className="text-fg-0">Bash</span>
            <span className="text-fg-2">
              (traceway logs query --trace-id 9f2c41d8)
            </span>
          </span>
        </div>
        <div className="grid grid-cols-[14px_1fr] gap-x-2">
          <span />
          <span className="whitespace-pre-wrap text-fg-3">
            {"⎿  12 log records · cart expired 09:13, reused 09:14"}
          </span>
        </div>

        <div className="mt-1 grid grid-cols-[14px_1fr] gap-x-2">
          <span className="text-ok">⏺</span>
          <span className="whitespace-pre-wrap">
            <span className="text-fg-0">Read</span>
            <span className="text-fg-2">(src/checkout/session.ts)</span>
          </span>
        </div>
        <div className="grid grid-cols-[14px_1fr] gap-x-2">
          <span />
          <span className="whitespace-pre-wrap text-fg-3">
            {"⎿  Read 86 lines"}
          </span>
        </div>

        <div className="mt-1 grid grid-cols-[14px_1fr] gap-x-2">
          <span className="text-fg-0">⏺</span>
          <span className="whitespace-pre-wrap text-fg-1">
            Root cause: repeat purchase reuses an expired cart. Fix ready for
            review in src/checkout/session.ts:42.
          </span>
        </div>

        <div className="mt-1 grid grid-cols-[14px_1fr] gap-x-2">
          <span className="text-fg-3">❯</span>
          <span>
            <span className="cursor bg-(--fg-2)" aria-hidden />
          </span>
        </div>
      </div>
    </div>
  );
}
