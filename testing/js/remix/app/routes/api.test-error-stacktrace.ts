import { withTraceway, traceway } from "~/lib/traceway";

export const loader = withTraceway(async (_request, { traceId }) => {
  const err = new Error("error with stack trace");
  traceway.captureExceptionWithAttributes(err, undefined, traceId);
  return new Response(JSON.stringify({ error: "stacktrace error" }), {
    status: 500,
    headers: { "Content-Type": "application/json" },
  });
});
