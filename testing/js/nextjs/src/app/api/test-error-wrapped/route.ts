import { NextResponse } from "next/server";
import * as traceway from "@traceway/backend";
import { withTraceway } from "@/lib/traceway-handler";

export const GET = withTraceway(async (_req, { traceId }) => {
  const base = new Error("base error");
  const wrapped = new Error("layer 1: " + base.message, { cause: base });
  const wrapped2 = new Error("layer 2: " + wrapped.message, { cause: wrapped });
  traceway.captureExceptionWithAttributes(wrapped2, undefined, traceId);
  return NextResponse.json({ error: "wrapped error" }, { status: 500 });
});
