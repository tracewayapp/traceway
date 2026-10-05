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
    <div className="grid gap-4 sm:grid-cols-2">
      {ITEMS.map((item) => (
        <div
          key={item.name}
          className="flex items-start gap-3.5 rounded-2xl p-5"
          style={{
            background: "var(--ink-0)",
            border: "1px solid var(--hair-2)",
            boxShadow:
              "0 1px 2px rgba(10,14,24,0.04), 0 12px 30px -18px rgba(10,14,24,0.14)",
          }}
        >
          <div
            className="grid size-10 shrink-0 place-items-center rounded-[10px]"
            style={{
              background: "color-mix(in oklab, var(--a2) 10%, transparent)",
            }}
          >
            <item.icon
              className="size-5"
              style={{ color: "var(--a2)" }}
              aria-hidden
            />
          </div>
          <div className="min-w-0 flex-1">
            <div className="flex flex-wrap items-center justify-between gap-x-2 gap-y-1.5">
              <div
                className="text-[15px] font-semibold leading-6"
                style={{ color: "var(--fg-0)" }}
              >
                {item.name}
              </div>
              <StatusPill status={item.status} label={item.statusLabel} />
            </div>
            <p
              className="mt-1 text-[13px] leading-snug"
              style={{ color: "var(--fg-2)" }}
            >
              {item.detail}
            </p>
          </div>
        </div>
      ))}
    </div>
  );
}

function StatusPill({ status, label }: { status: Status; label: string }) {
  const isReady = status === "ready";
  return (
    <span
      className="inline-flex items-center whitespace-nowrap rounded-full px-2.5 py-0.5 text-[11.5px] font-medium"
      style={
        isReady
          ? {
              color: "var(--ok)",
              background: "color-mix(in oklab, var(--ok) 12%, transparent)",
            }
          : { color: "var(--fg-2)", background: "var(--ink-2)" }
      }
    >
      {label}
    </span>
  );
}
