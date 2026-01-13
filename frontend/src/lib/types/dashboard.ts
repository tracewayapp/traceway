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
	status: MetricStatus;
	formatValue?: (value: number) => string;
	servers?: ServerMetricTrend[]; // Per-server breakdown for multi-server metrics
};

export type DashboardData = {
	metrics: DashboardMetric[];
	lastUpdated: Date;
	availableServers?: string[]; // List of servers with data in the time range
};

// Tab types for split metrics endpoints
export type MetricsTab = 'application' | 'stats' | 'server';

// Response type for /api/metrics/application
export type ApplicationMetricsData = {
	metrics: DashboardMetric[];
	availableServers: string[];
	lastUpdated: Date;
};

// Response type for /api/metrics/stats
export type StatsMetricsData = {
	metrics: DashboardMetric[];
	lastUpdated: Date;
};

// Response type for /api/metrics/server
export type ServerMetricsData = {
	metrics: DashboardMetric[];
	availableServers: string[];
	lastUpdated: Date;
};
