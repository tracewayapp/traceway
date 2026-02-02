import { NextResponse } from "next/server";
import * as traceway from "@traceway/backend";
import { withTraceway } from "@/lib/traceway-handler";

export const GET = withTraceway(async () => {
  traceway.measureTask("background-data-processor", async () => {
    const span = traceway.startSpan("processing");
    await new Promise((r) => setTimeout(r, 200));
    traceway.endSpan(span);
  });
  return NextResponse.json({ status: "task started" });
});
