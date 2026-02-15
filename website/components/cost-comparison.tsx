import { DollarSign, Database, ShieldCheck } from "lucide-react";
import {
  Card,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";

const cards = [
  {
    icon: DollarSign,
    iconBg: "bg-green-50",
    iconColor: "text-green-600",
    title: "No per-event pricing",
    description:
      "Competitors charge you more as you grow. Traceway runs on your infrastructure with predictable, fixed costs.",
  },
  {
    icon: Database,
    iconBg: "bg-blue-50",
    iconColor: "text-blue-600",
    title: "ClickHouse compression",
    description:
      "Columnar storage compresses 1 million daily events into ~2-3 GB per month. Your storage bill stays tiny.",
  },
  {
    icon: ShieldCheck,
    iconBg: "bg-orange-50",
    iconColor: "text-orange-600",
    title: "Fixed costs, not surprises",
    description:
      "A single server handles millions of events. No metered billing, no overage charges, no surprises.",
  },
];

export function CostComparison() {
  return (
    <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
      {cards.map((card) => (
        <Card
          key={card.title}
          className="bg-white border-zinc-200 transition-all duration-300"
        >
          <CardHeader className="p-6 pt-0">
            <div
              className={`w-10 h-10 ${card.iconBg} rounded-lg flex items-center justify-center mb-3`}
            >
              <card.icon className={`w-5 h-5 ${card.iconColor}`} />
            </div>
            <CardTitle className="text-lg">{card.title}</CardTitle>
            <CardDescription className="text-zinc-500 text-sm mt-1.5">
              {card.description}
            </CardDescription>
          </CardHeader>
        </Card>
      ))}
    </div>
  );
}
