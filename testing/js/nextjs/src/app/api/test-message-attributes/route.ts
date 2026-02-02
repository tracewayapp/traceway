import { NextResponse } from "next/server";
import * as traceway from "@traceway/backend";
import { withTraceway } from "@/lib/traceway-handler";

export const GET = withTraceway(async () => {
  traceway.captureMessage("test message with attributes");
  return NextResponse.json({ status: "message with attributes sent" });
});
