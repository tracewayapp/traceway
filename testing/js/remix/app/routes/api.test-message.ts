import { withTraceway, traceway } from "~/lib/traceway";

export const loader = withTraceway(async () => {
  traceway.captureMessage("test message from /test-message");
  return new Response(JSON.stringify({ status: "message sent" }), {
    headers: { "Content-Type": "application/json" },
  });
});
