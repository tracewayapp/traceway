export function spanIdUuidToHex(spanUuid: string | null | undefined): string {
	if (!spanUuid) return '';
	return spanUuid.replace(/-/g, '').slice(-16).toLowerCase();
}

export function traceIdUuidToHex(traceUuid: string | null | undefined): string {
	if (!traceUuid) return '';
	return traceUuid.replace(/-/g, '').toLowerCase();
}

export function logTraceId(trace: {
	id: string;
	distributedTraceId?: string | null;
	attributes?: Record<string, string> | null;
}): string {
	const otelTraceId = trace.attributes?.['traceway.otel.trace_id'];
	if (otelTraceId) return traceIdUuidToHex(otelTraceId);

	const occurrenceId = traceIdUuidToHex(trace.id);
	// Older OTel non-root occurrences store a zero-padded span ID as their ID.
	// Native traces keep their own trace ID, even when grouped into a distributed trace.
	if (occurrenceId.startsWith('0000000000000000') && trace.distributedTraceId) {
		return traceIdUuidToHex(trace.distributedTraceId);
	}
	return occurrenceId;
}
