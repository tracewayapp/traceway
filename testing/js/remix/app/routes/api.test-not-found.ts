import { withTraceway } from "~/lib/traceway";

export const loader = withTraceway(async () => {
  return new Response(JSON.stringify({ status: "not-found" }), {
    status: 404,
    headers: { "Content-Type": "application/json" },
  });
});
