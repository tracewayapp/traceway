import { withTraceway, traceway } from "~/lib/traceway";

export const loader = withTraceway(async () => {
  traceway.captureMetric("test.custom_metric", 42.0);
  return new Response(JSON.stringify({ status: "metric captured" }), {
    headers: { "Content-Type": "application/json" },
  });
});
