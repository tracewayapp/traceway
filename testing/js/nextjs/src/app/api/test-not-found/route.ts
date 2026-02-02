import { NextResponse } from "next/server";
import { withTraceway } from "@/lib/traceway-handler";

export const GET = withTraceway(async () => {
  return NextResponse.json({ status: "not-found" }, { status: 404 });
});
