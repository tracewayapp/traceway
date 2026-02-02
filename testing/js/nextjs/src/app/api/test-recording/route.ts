import { NextRequest, NextResponse } from "next/server";
import { withTraceway } from "@/lib/traceway-handler";

export const POST = withTraceway(async (req: NextRequest) => {
  const body = await req.json();

  if (body?.action === "panic") {
    throw new Error("panic triggered by /test-recording");
  }

  return NextResponse.json({ status: "ok", received: body });
});
