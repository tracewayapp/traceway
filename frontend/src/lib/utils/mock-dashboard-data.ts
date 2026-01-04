import type { DashboardData, MetricTrendPoint, MetricStatus } from '$lib/types/dashboard';

function generateTrendData(
	baseValue: number,
	points: number = 12,
	variance: number = 0.1
): MetricTrendPoint[] {
	const now = Date.now();
	const interval = 60000; // 1 minute intervals

	return Array.from({ length: points }, (_, i) => ({
		timestamp: new Date(now - (points - i - 1) * interval),
		value: baseValue + (Math.random() - 0.5) * baseValue * variance
	}));
}

function determineStatus(
	value: number,
	thresholds: { warning: number; critical: number },
	inverted: boolean = false
): MetricStatus {
	if (inverted) {
		if (value < thresholds.critical) return 'critical';
		if (value < thresholds.warning) return 'warning';
		return 'healthy';
	} else {
		if (value > thresholds.critical) return 'critical';
		if (value > thresholds.warning) return 'warning';
		return 'healthy';
	}
}

export function generateMockDashboardData(): DashboardData {
	const cpuValue = 45 + Math.random() * 20; // 45-65%
	const memoryValue = 512 + Math.random() * 256; // 512-768 MB
	const goRoutinesValue = 150 + Math.random() * 100; // 150-250
	const heapObjectsValue = 25000 + Math.random() * 10000; // 25k-35k
	const errorRateValue = 0.5 + Math.random() * 2; // 0.5-2.5%
	const p95LatencyValue = 120 + Math.random() * 80; // 120-200ms
	const throughputValue = 850 + Math.random() * 300; // 850-1150 req/s
	const apdexValue = 0.85 + Math.random() * 0.1; // 0.85-0.95

	return {
		metrics: [
			{
				id: 'cpu',
				name: 'CPU Usage',
				value: cpuValue,
				unit: '%',
				trend: generateTrendData(cpuValue, 12),
				change24h: -2.3 + Math.random() * 4, // -2.3 to 1.7
				status: determineStatus(cpuValue, { warning: 70, critical: 90 }),
				formatValue: (v) => v.toFixed(1)
			},
			{
				id: 'memory',
				name: 'Memory',
				value: memoryValue,
				unit: 'MB',
				trend: generateTrendData(memoryValue, 12),
				change24h: 5.8 + Math.random() * 4 - 2, // 3.8 to 7.8
				status: determineStatus(memoryValue, { warning: 700, critical: 900 }),
				formatValue: (v) => Math.round(v).toString()
			},
			{
				id: 'goroutines',
				name: 'Go Routines',
				value: goRoutinesValue,
				unit: '',
				trend: generateTrendData(goRoutinesValue, 12),
				change24h: 12.4 + Math.random() * 4 - 2, // 10.4 to 14.4
				status: determineStatus(goRoutinesValue, { warning: 300, critical: 500 }),
				formatValue: (v) => Math.round(v).toString()
			},
			{
				id: 'heap',
				name: 'Heap Objects',
				value: heapObjectsValue,
				unit: '',
				trend: generateTrendData(heapObjectsValue, 12),
				change24h: -1.2 + Math.random() * 4 - 2, // -3.2 to 0.8
				status: determineStatus(heapObjectsValue, { warning: 40000, critical: 60000 }),
				formatValue: (v) => (v / 1000).toFixed(1) + 'k'
			},
			{
				id: 'error-rate',
				name: 'Error Rate',
				value: errorRateValue,
				unit: '%',
				trend: generateTrendData(errorRateValue, 12),
				change24h: -15.3 + Math.random() * 10, // -15.3 to -5.3
				status: determineStatus(errorRateValue, { warning: 2, critical: 5 }),
				formatValue: (v) => v.toFixed(2)
			},
			{
				id: 'p95-latency',
				name: 'P95 Latency',
				value: p95LatencyValue,
				unit: 'ms',
				trend: generateTrendData(p95LatencyValue, 12),
				change24h: 3.7 + Math.random() * 4 - 2, // 1.7 to 5.7
				status: determineStatus(p95LatencyValue, { warning: 200, critical: 500 }),
				formatValue: (v) => Math.round(v).toString()
			},
			{
				id: 'throughput',
				name: 'Throughput',
				value: throughputValue,
				unit: 'req/s',
				trend: generateTrendData(throughputValue, 12),
				change24h: 8.2 + Math.random() * 4 - 2, // 6.2 to 10.2
				status: determineStatus(throughputValue, { warning: 500, critical: 300 }, true),
				formatValue: (v) => Math.round(v).toString()
			},
			{
				id: 'apdex',
				name: 'Apdex Score',
				value: apdexValue,
				unit: '',
				trend: generateTrendData(apdexValue, 12, 0.05),
				change24h: 1.8 + Math.random() * 2 - 1, // 0.8 to 2.8
				status: determineStatus(apdexValue, { warning: 0.7, critical: 0.5 }, true),
				formatValue: (v) => v.toFixed(3)
			}
		],
		lastUpdated: new Date()
	};
}
