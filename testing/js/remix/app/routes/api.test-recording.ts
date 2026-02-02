import { withTraceway } from "~/lib/traceway";

export const action = withTraceway(async (request) => {
  const body = await request.json();

  if (body?.action === "panic") {
    throw new Error("panic triggered by /test-recording");
  }

  return new Response(JSON.stringify({ status: "ok", received: body }), {
    headers: { "Content-Type": "application/json" },
  });
});
