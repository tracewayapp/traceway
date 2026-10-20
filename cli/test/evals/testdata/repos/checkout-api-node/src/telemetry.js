// Thin wrapper over @opentelemetry/api so modules can annotate the active
// span; it degrades to no-ops when the API is not installed (unit tests).
let api = null;
try {
  api = require('@opentelemetry/api');
} catch {
  api = null;
}

function setAttributes(attrs) {
  api?.trace.getActiveSpan()?.setAttributes(attrs);
}

function recordException(err) {
  const span = api?.trace.getActiveSpan();
  if (!span) return;
  span.recordException(err);
  span.setStatus({ code: api.SpanStatusCode.ERROR, message: err.message });
}

module.exports = { setAttributes, recordException };
