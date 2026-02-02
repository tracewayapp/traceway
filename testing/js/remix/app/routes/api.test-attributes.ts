import { withTraceway, traceway } from "~/lib/traceway";

export const loader = withTraceway(async () => {
  traceway.captureExceptionWithAttributes(
    new Error("exception with custom attributes"),
    {
      user_id: "usr_123",
      request_id: "req_456",
      env: "testing",
    }
  );
  return new Response(JSON.stringify({ status: "exception with attributes captured" }), {
    headers: { "Content-Type": "application/json" },
  });
});
