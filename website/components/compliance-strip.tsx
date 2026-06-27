import { ShieldCheck } from "lucide-react";

type Status = "in-progress" | "ready";

type Item = {
  icon: typeof ShieldCheck;
  name: string;
  detail: string;
  status: Status;
  statusLabel: string;
};

const ITEMS: Item[] = [
  {
    icon: ShieldCheck,
    name: "SOC 2 Type II",
    detail: "Security, availability & confidentiality controls",
    status: "in-progress",
    statusLabel: "In progress",
  },
  {
    icon: ShieldCheck,
    name: "ISO 27001",
    detail: "Information security management system",
    status: "in-progress",
    statusLabel: "In progress",
  },
];

export function ComplianceStrip() {
  return (
    <div className="grid gap-5 sm:grid-cols-2">
      {ITEMS.map((item) => (
        <div
          key={item.name}
          className="flex items-start gap-4 rounded-2xl p-5"
          style={{
            background: "var(--ink-0)",
            border: "1px solid var(--hair-2)",
            boxShadow:
              "0 1px 2px rgba(10,14,24,0.04), 0 12px 30px -18px rgba(10,14,24,0.14)",
          }}
        >
          <div className="grid size-10 shrink-0 place-items-center rounded-xl">
            <item.icon
              className="size-5"
              style={{ color: "var(--a2)" }}
              aria-hidden
            />
          </div>
          <div className="min-w-0">
            <div
              className="text-[15px] font-semibold"
              style={{ color: "var(--fg-0)" }}
            >
              {item.name}
            </div>
            <p
              className="mt-0.5 text-[12.5px] leading-snug"
              style={{ color: "var(--fg-2)" }}
            >
              {item.detail}
            </p>
            <div className="mt-2.5">
              <StatusPill status={item.status} label={item.statusLabel} />
            </div>
          </div>
        </div>
      ))}
    </div>
  );
}

function StatusPill({ status, label }: { status: Status; label: string }) {
  const isReady = status === "ready";
  const tone = "var(--ok)";
  return (
    <span
      className="inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-[11px] font-medium"
      style={{
        fontFamily: "var(--font-mono)",
        letterSpacing: "0.04em",
        color: tone,
        border: "1px solid var(--ok)",
      }}
    >
      <span className="relative flex size-1.5">
        {!isReady && (
          <span
            className="absolute inline-flex size-full animate-ping rounded-full opacity-60"
            style={{ background: tone }}
          />
        )}
        <span
          className="relative inline-flex size-1.5 rounded-full"
          style={{ background: tone }}
        />
      </span>
      {label}
    </span>
  );
}
