import { NextResponse } from "next/server";
import * as traceway from "@traceway/backend";
import { withTraceway } from "@/lib/traceway-handler";

export const GET = withTraceway(async (_req, { startSpan }) => {
  const dbSpan = startSpan("db.query");
  await new Promise((r) => setTimeout(r, 50));
  traceway.endSpan(dbSpan);

  const cacheSpan = startSpan("cache.set");
  await new Promise((r) => setTimeout(r, 20));
  traceway.endSpan(cacheSpan);

  const httpSpan = startSpan("http.external_api");
  await new Promise((r) => setTimeout(r, 100));
  traceway.endSpan(httpSpan);

  return NextResponse.json({ status: "spans captured" });
});
