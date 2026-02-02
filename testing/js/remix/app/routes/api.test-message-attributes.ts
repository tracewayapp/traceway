import { withTraceway, traceway } from "~/lib/traceway";

export const loader = withTraceway(async () => {
  // JS SDK captureMessage does not support attributes
  traceway.captureMessage("test message with attributes");
  return new Response(JSON.stringify({ status: "message with attributes sent" }), {
    headers: { "Content-Type": "application/json" },
  });
});
