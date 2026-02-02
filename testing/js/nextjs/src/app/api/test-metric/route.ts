import { NextResponse } from "next/server";
import * as traceway from "@traceway/backend";
import { withTraceway } from "@/lib/traceway-handler";

export const GET = withTraceway(async () => {
  traceway.captureMetric("test.custom_metric", 42.0);
  return NextResponse.json({ status: "metric captured" });
});
