export type MetricStatus = 'healthy' | 'warning' | 'critical';

export type MetricTrendPoint = {
	timestamp: Date;
	value: number;
};

export type ServerMetricTrend = {
	serverName: string;
	value: number;
	trend: MetricTrendPoint[];
};

export type DashboardMetric = {
	id: string;
	name: string;
	value: number;
	unit: string;
	trend: MetricTrendPoint[];
	change24h: number; // Percentage change (e.g., 5.2 or -3.1)
	status: MetricStatus;
	formatValue?: (value: number) => string;
	servers?: ServerMetricTrend[]; // Per-server breakdown for multi-server metrics
};

export type DashboardData = {
	metrics: DashboardMetric[];
	lastUpdated: Date;
	availableServers?: string[]; // List of servers with data in the time range
};
