import { DollarSign, Database, ShieldCheck } from "lucide-react";

const cards = [
  {
    icon: DollarSign,
    color: "var(--ok)",
    title: "No per-event pricing",
    description:
      "Competitors charge more as you grow. Self-host on your own infrastructure or use Cloud with fixed-price tiers. No per-event, per-host, or per-seat fees.",
  },
  {
    icon: Database,
    color: "var(--a2)",
    title: "ClickHouse compression",
    description:
      "Columnar storage compresses 1 million daily events into ~2-3 GB per month. Your storage bill stays tiny.",
  },
  {
    icon: ShieldCheck,
    color: "var(--a4)",
    title: "Fixed costs, not surprises",
    description:
      "Every plan has a fixed monthly price. No metered billing, no overage charges, no surprise line items. Your bill never increases without your approval.",
  },
];

export function CostComparison() {
  return (
    <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
      {cards.map((card) => {
        const Icon = card.icon;
        return (
          <div
            key={card.title}
            className="surface-card"
            style={{
              padding: 24,
            }}
          >
            <div
              className="w-10 h-10 rounded-[8px] flex items-center justify-center mb-4"
              style={{
                background: `color-mix(in oklab, ${card.color} 18%, transparent)`,
                border: `1px solid color-mix(in oklab, ${card.color} 40%, transparent)`,
                color: card.color,
              }}
            >
              <Icon className="w-5 h-5" />
            </div>
            <div
              className="text-[17px] font-semibold mb-2"
              style={{ color: "var(--fg-0)", fontFamily: "var(--font-display)" }}
            >
              {card.title}
            </div>
            <div className="text-[14px]" style={{ color: "var(--fg-2)", lineHeight: 1.55 }}>
              {card.description}
            </div>
          </div>
        );
      })}
    </div>
  );
}
