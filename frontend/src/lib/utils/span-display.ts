import type { Span } from '$lib/types/spans';

const STATEMENT_PREVIEW_LIMIT = 200;

export function spanDisplayName(span: Span): string {
	const statement =
		span.attributes?.['db.query.text'] || span.attributes?.['db.statement'] || span.dbStatement;
	if (!statement) return span.name;
	const preview = statement.replace(/\s+/g, ' ').trim();
	if (!preview) return span.name;
	return preview.length > STATEMENT_PREVIEW_LIMIT
		? `${preview.slice(0, STATEMENT_PREVIEW_LIMIT)}…`
		: preview;
}
