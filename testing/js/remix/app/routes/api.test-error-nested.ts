import { withTraceway, traceway } from "~/lib/traceway";

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

export const loader = withTraceway(async (_request, { traceId }) => {
  const err = outerFunction();
  traceway.captureExceptionWithAttributes(err, undefined, traceId);
  return new Response(JSON.stringify({ error: "nested error" }), {
    status: 500,
    headers: { "Content-Type": "application/json" },
  });
});
