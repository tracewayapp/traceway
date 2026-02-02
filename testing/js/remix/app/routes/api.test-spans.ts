import { withTraceway, traceway } from "~/lib/traceway";

export const loader = withTraceway(async (_request, { startSpan }) => {
  const dbSpan = startSpan("db.query");
  await new Promise((r) => setTimeout(r, 50));
  traceway.endSpan(dbSpan);

  const cacheSpan = startSpan("cache.set");
  await new Promise((r) => setTimeout(r, 20));
  traceway.endSpan(cacheSpan);

  const httpSpan = startSpan("http.external_api");
  await new Promise((r) => setTimeout(r, 100));
  traceway.endSpan(httpSpan);

  return new Response(JSON.stringify({ status: "spans captured" }), {
    headers: { "Content-Type": "application/json" },
  });
});
