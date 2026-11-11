// Generated data lives in content/services-pricing.json.
// Edit it in the traceway-marketing repo and run `npm run sync` there.
import data from "@/content/services-pricing.json";

export const servicesPricing = data;

export function money(amount: number): string {
  return `$${amount.toLocaleString("en-US", {
    minimumFractionDigits: Number.isInteger(amount) ? 0 : 2,
    maximumFractionDigits: 2,
  })}`;
}
