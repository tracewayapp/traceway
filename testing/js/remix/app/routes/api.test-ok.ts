import { withTraceway } from "~/lib/traceway";

export const loader = withTraceway(async () => {
  return new Response(JSON.stringify({ status: "ok" }), {
    headers: { "Content-Type": "application/json" },
  });
});
