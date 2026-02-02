import { withTraceway, traceway } from "~/lib/traceway";

export const loader = withTraceway(async (_request, { traceId }) => {
  const base = new Error("base error");
  const wrapped = new Error("layer 1: " + base.message, { cause: base });
  const wrapped2 = new Error("layer 2: " + wrapped.message, { cause: wrapped });
  traceway.captureExceptionWithAttributes(wrapped2, undefined, traceId);
  return new Response(JSON.stringify({ error: "wrapped error" }), {
    status: 500,
    headers: { "Content-Type": "application/json" },
  });
});
