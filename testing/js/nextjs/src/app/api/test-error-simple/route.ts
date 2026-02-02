import { NextResponse } from "next/server";
import * as traceway from "@traceway/backend";
import { withTraceway } from "@/lib/traceway-handler";

export const GET = withTraceway(async (_req, { traceId }) => {
  const err = new Error("simple error without stack");
  traceway.captureExceptionWithAttributes(err, undefined, traceId);
  return NextResponse.json({ error: "simple error" }, { status: 500 });
});
