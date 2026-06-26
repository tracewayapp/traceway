export interface ProfileTypeInfo {
	type: string;
	unit: string;
	isGauge: boolean;
}

function trim(value: number): string {
	return Number(value.toFixed(2)).toString();
}

function formatNanoseconds(ns: number): string {
	if (ns < 1_000) return `${trim(ns)} ns`;
	if (ns < 1_000_000) return `${trim(ns / 1_000)} µs`;
	if (ns < 1_000_000_000) return `${trim(ns / 1_000_000)} ms`;
	return `${trim(ns / 1_000_000_000)} s`;
}

function formatBytes(bytes: number): string {
	if (bytes < 1024) return `${trim(bytes)} B`;
	if (bytes < 1024 ** 2) return `${trim(bytes / 1024)} KB`;
	if (bytes < 1024 ** 3) return `${trim(bytes / 1024 ** 2)} MB`;
	return `${trim(bytes / 1024 ** 3)} GB`;
}

const TIME_TO_NS: Record<string, number> = {
	nanoseconds: 1,
	nanos: 1,
	ns: 1,
	microseconds: 1_000,
	micros: 1_000,
	us: 1_000,
	µs: 1_000,
	milliseconds: 1_000_000,
	millis: 1_000_000,
	ms: 1_000_000,
	seconds: 1_000_000_000,
	s: 1_000_000_000
};

const BYTE_UNITS = new Set(['bytes', 'byte', 'b']);
const COUNT_UNITS = new Set(['count', 'samples', 'sample', 'frames', 'objects', 'goroutines', 'none', '']);

export function formatValue(unit: string, value: number): string {
	const u = (unit || '').toLowerCase();
	if (u in TIME_TO_NS) return formatNanoseconds(value * TIME_TO_NS[u]);
	if (BYTE_UNITS.has(u)) return formatBytes(value);
	if (COUNT_UNITS.has(u)) return Math.round(value).toLocaleString();
	return `${value.toLocaleString()} ${unit}`.trim();
}

const KNOWN_LABELS: Record<string, string> = {
	cpu: 'CPU',
	inuse_space: 'In-Use',
	alloc_space: 'Alloc',
	inuse_objects: 'In-Use Obj',
	alloc_objects: 'Alloc Obj',
	goroutine: 'Goroutines',
	mutex: 'Mutex',
	block: 'Block',
	samples: 'Samples',
	wall: 'Wall',
	lock: 'Lock'
};

export function humanizeType(type: string): string {
	if (KNOWN_LABELS[type]) return KNOWN_LABELS[type];
	const tail = type.includes(':') ? (type.split(':').pop() ?? type) : type;
	return tail
		.replace(/[._]/g, ' ')
		.split(' ')
		.filter(Boolean)
		.map((w) => w.charAt(0).toUpperCase() + w.slice(1))
		.join(' ');
}

export function defaultProfileType(types: string[]): string {
	if (types.includes('cpu')) return 'cpu';
	return types[0] ?? '';
}

export function isTimeUnit(unit: string): boolean {
	return (unit || '').toLowerCase() in TIME_TO_NS;
}

export function formatRate(unit: string, value: number): string {
	const u = (unit || '').toLowerCase();
	if (u in TIME_TO_NS) {
		const cores = (value * TIME_TO_NS[u]) / 1_000_000_000;
		return `${trim(cores)} cores`;
	}
	if (BYTE_UNITS.has(u)) return `${formatBytes(value)}/s`;
	if (COUNT_UNITS.has(u)) return `${Math.round(value).toLocaleString()}/s`;
	return `${value.toLocaleString()} ${unit}/s`.trim();
}
