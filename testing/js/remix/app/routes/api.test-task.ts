import { withTraceway, traceway } from "~/lib/traceway";

export const loader = withTraceway(async () => {
  traceway.measureTask("background-data-processor", async () => {
    const span = traceway.startSpan("processing");
    await new Promise((r) => setTimeout(r, 200));
    traceway.endSpan(span);
  });
  return new Response(JSON.stringify({ status: "task started" }), {
    headers: { "Content-Type": "application/json" },
  });
});
