import { NextResponse } from "next/server";
import * as traceway from "@traceway/backend";
import { withTraceway } from "@/lib/traceway-handler";

function innerFunction(): Error {
  const err = new Error("error from inner function");
  Error.captureStackTrace(err, innerFunction);
  return err;
}

function middleFunction(): Error {
  return innerFunction();
}

function outerFunction(): Error {
  return middleFunction();
}

export const GET = withTraceway(async (_req, { traceId }) => {
  const err = outerFunction();
  traceway.captureExceptionWithAttributes(err, undefined, traceId);
  return NextResponse.json({ error: "nested error" }, { status: 500 });
});
