type FormatResult = { text: string; unit: string };

export function formatMetricValue(value: number, unit: string): FormatResult {
	if (unit.length > 2 && unit.endsWith('/s')) {
		const base = formatMetricValue(value, unit.slice(0, -2));
		return { text: base.text, unit: `${base.unit}/s` };
	}

	// OTel canonical unit '1' is a dimensionless ratio (0-1) - render as percent.
	if (unit === '1') {
		return formatMetricValue(value * 100, '%');
	}

	// OTel canonical unit 'By' is bytes - normalize to the existing bytes branch.
	if (unit === 'By') {
		return formatMetricValue(value, 'B');
	}

	// OTel annotation units like '{connections}', '{packets}' - treat as raw count.
	if (unit.length > 2 && unit.startsWith('{') && unit.endsWith('}')) {
		return formatMetricValue(value, 'count');
	}

	if (unit === '%') {
		if (value === 0) return { text: '0', unit: '%' };
		if (Math.abs(value) < 0.1) return { text: value.toFixed(2), unit: '%' };
		if (Math.abs(value) < 10) return { text: value.toFixed(1), unit: '%' };
		return { text: Math.round(value).toString(), unit: '%' };
	}

	if (unit === 'ms') {
		if (value < 1) return { text: (value * 1000).toFixed(0), unit: 'µs' };
		if (value < 10) return { text: value.toFixed(1), unit: 'ms' };
		if (value < 1000) return { text: Math.round(value).toString(), unit: 'ms' };
		return { text: (value / 1000).toFixed(1), unit: 's' };
	}

	if (unit === 'ns') {
		if (value < 1000) return { text: Math.round(value).toString(), unit: 'ns' };
		if (value < 1_000_000) return { text: (value / 1000).toFixed(1), unit: 'µs' };
		if (value < 1_000_000_000) return { text: (value / 1_000_000).toFixed(1), unit: 'ms' };
		return { text: (value / 1_000_000_000).toFixed(1), unit: 's' };
	}

	if (unit === 's') {
		if (value < 0.001) return { text: (value * 1_000_000).toFixed(0), unit: 'µs' };
		if (value < 1) return { text: (value * 1000).toFixed(1), unit: 'ms' };
		if (value < 60) return { text: value.toFixed(1), unit: 's' };
		if (value < 3600) return { text: (value / 60).toFixed(1), unit: 'min' };
		return { text: (value / 3600).toFixed(1), unit: 'h' };
	}

	if (unit === 'MB') {
		if (value < 1) return { text: (value * 1024).toFixed(0), unit: 'KB' };
		if (value >= 1024) return { text: (value / 1024).toFixed(1), unit: 'GB' };
		return { text: Math.round(value).toString(), unit: 'MB' };
	}

	if (unit === 'GB') {
		if (value < 1) return { text: (value * 1024).toFixed(0), unit: 'MB' };
		if (value >= 1024) return { text: (value / 1024).toFixed(1), unit: 'TB' };
		return { text: value.toFixed(1), unit: 'GB' };
	}

	if (unit === 'bytes' || unit === 'B') {
		if (value < 1024) return { text: Math.round(value).toString(), unit: 'B' };
		if (value < 1024 * 1024) return { text: (value / 1024).toFixed(1), unit: 'KB' };
		if (value < 1024 * 1024 * 1024) return { text: (value / (1024 * 1024)).toFixed(1), unit: 'MB' };
		return { text: (value / (1024 * 1024 * 1024)).toFixed(1), unit: 'GB' };
	}

	if (unit === 'count' || unit === '') {
		if (value >= 1_000_000) return { text: (value / 1_000_000).toFixed(1), unit: 'M' };
		if (value >= 1_000) return { text: (value / 1_000).toFixed(1), unit: 'K' };
		return { text: Math.round(value).toString(), unit: '' };
	}

	if (Number.isInteger(value)) return { text: value.toString(), unit };
	return { text: value.toFixed(1), unit };
}

export function formatMetricLabel(value: number, unit: string): string {
	const result = formatMetricValue(value, unit);
	if (!result.unit) return result.text;
	return `${result.text} ${result.unit}`;
}

type AxisScale = { divisor: number; unit: string };

export function getMetricAxisScale(maxValue: number, unit: string): AxisScale {
	if (unit.length > 2 && unit.endsWith('/s')) {
		const base = getMetricAxisScale(maxValue, unit.slice(0, -2));
		return { divisor: base.divisor, unit: `${base.unit}/s` };
	}

	if (unit === '1') {
		return { divisor: 0.01, unit: '%' };
	}

	if (unit === 'By') {
		return getMetricAxisScale(maxValue, 'B');
	}

	if (unit.length > 2 && unit.startsWith('{') && unit.endsWith('}')) {
		return getMetricAxisScale(maxValue, 'count');
	}

	if (unit === '%') {
		return { divisor: 1, unit: '%' };
	}

	if (unit === 'ms') {
		if (maxValue < 1) return { divisor: 0.001, unit: 'µs' };
		if (maxValue < 1000) return { divisor: 1, unit: 'ms' };
		return { divisor: 1000, unit: 's' };
	}

	if (unit === 'ns') {
		if (maxValue < 1000) return { divisor: 1, unit: 'ns' };
		if (maxValue < 1_000_000) return { divisor: 1000, unit: 'µs' };
		if (maxValue < 1_000_000_000) return { divisor: 1_000_000, unit: 'ms' };
		return { divisor: 1_000_000_000, unit: 's' };
	}

	if (unit === 's') {
		if (maxValue < 0.001) return { divisor: 0.000001, unit: 'µs' };
		if (maxValue < 1) return { divisor: 0.001, unit: 'ms' };
		if (maxValue < 60) return { divisor: 1, unit: 's' };
		if (maxValue < 3600) return { divisor: 60, unit: 'min' };
		return { divisor: 3600, unit: 'h' };
	}

	if (unit === 'MB') {
		if (maxValue < 1) return { divisor: 1 / 1024, unit: 'KB' };
		if (maxValue >= 1024) return { divisor: 1024, unit: 'GB' };
		return { divisor: 1, unit: 'MB' };
	}

	if (unit === 'GB') {
		if (maxValue < 1) return { divisor: 1 / 1024, unit: 'MB' };
		if (maxValue >= 1024) return { divisor: 1024, unit: 'TB' };
		return { divisor: 1, unit: 'GB' };
	}

	if (unit === 'bytes' || unit === 'B') {
		if (maxValue < 1024) return { divisor: 1, unit: 'B' };
		if (maxValue < 1024 * 1024) return { divisor: 1024, unit: 'KB' };
		if (maxValue < 1024 * 1024 * 1024) return { divisor: 1024 * 1024, unit: 'MB' };
		return { divisor: 1024 * 1024 * 1024, unit: 'GB' };
	}

	if (unit === 'count' || unit === '') {
		if (maxValue >= 1_000_000) return { divisor: 1_000_000, unit: 'M' };
		if (maxValue >= 1_000) return { divisor: 1_000, unit: 'K' };
		return { divisor: 1, unit: '' };
	}

	return { divisor: 1, unit };
}

export function formatAxisTick(value: number, divisor: number): string {
	const scaled = value / divisor;
	if (Number.isInteger(scaled)) return scaled.toString();
	if (scaled !== 0 && Math.abs(scaled) < 0.1) return scaled.toFixed(2);
	return scaled.toFixed(1);
}
