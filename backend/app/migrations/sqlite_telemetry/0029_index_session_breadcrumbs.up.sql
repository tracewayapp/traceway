CREATE INDEX IF NOT EXISTS idx_log_records_session ON log_records(project_id, json_extract(log_attributes, '$."session.id"'), timestamp);

CREATE INDEX IF NOT EXISTS idx_spans_v2_session ON spans_v2(project_id, json_extract(span_attributes, '$."session.id"'), recorded_at);
