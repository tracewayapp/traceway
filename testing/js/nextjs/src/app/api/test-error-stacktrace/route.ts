import { NextResponse } from "next/server";
import * as traceway from "@traceway/backend";
import { withTraceway } from "@/lib/traceway-handler";

export const GET = withTraceway(async (_req, { traceId }) => {
  const err = new Error("error with stack trace");
  traceway.captureExceptionWithAttributes(err, undefined, traceId);
  return NextResponse.json({ error: "stacktrace error" }, { status: 500 });
});
