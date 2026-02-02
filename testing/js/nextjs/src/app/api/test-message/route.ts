import { NextResponse } from "next/server";
import * as traceway from "@traceway/backend";
import { withTraceway } from "@/lib/traceway-handler";

export const GET = withTraceway(async () => {
  traceway.captureMessage("test message from /test-message");
  return NextResponse.json({ status: "message sent" });
});
