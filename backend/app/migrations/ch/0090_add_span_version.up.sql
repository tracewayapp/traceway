ALTER TABLE spans_v2 ADD COLUMN IF NOT EXISTS span_version FixedString(32)
    MATERIALIZED SHA256(concat(toString(length(span_pb)), ':', span_pb,
        toString(length(resource_pb)), ':', resource_pb,
        toString(length(scope_pb)), ':', scope_pb))
