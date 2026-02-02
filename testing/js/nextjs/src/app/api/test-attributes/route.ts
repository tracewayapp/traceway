import { NextResponse } from "next/server";
import * as traceway from "@traceway/backend";
import { withTraceway } from "@/lib/traceway-handler";

export const GET = withTraceway(async () => {
  traceway.captureExceptionWithAttributes(
    new Error("exception with custom attributes"),
    {
      user_id: "usr_123",
      request_id: "req_456",
      env: "testing",
    }
  );
  return NextResponse.json({ status: "exception with attributes captured" });
});
