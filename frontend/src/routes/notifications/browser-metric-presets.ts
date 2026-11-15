export interface BrowserMetricPreset {
	id: string;
	label: string;
	ruleName: string;
	metricName: string;
	aggregation: string;
	operator: string;
	thresholdValue: number;
	lookbackMinutes: number;
	groupBy: string;
}

/**
 * Starting points for metric alerts on a browser project, on the metrics the backend derives from the SDK's
 * performance events. The thresholds are the "poor" boundaries of the web vitals, and a second for a freeze.
 */
export const BROWSER_METRIC_PRESETS: BrowserMetricPreset[] = [
	{
		id: 'slow-lcp',
		label: 'Slow LCP',
		ruleName: 'Slow page loads (LCP)',
		metricName: 'browser.web_vital.lcp',
		aggregation: 'p95',
		operator: 'gt',
		thresholdValue: 4000,
		lookbackMinutes: 15,
		groupBy: ''
	},
	{
		id: 'slow-inp',
		label: 'Slow interactions',
		ruleName: 'Slow interactions (INP)',
		metricName: 'browser.web_vital.inp',
		aggregation: 'p95',
		operator: 'gt',
		thresholdValue: 500,
		lookbackMinutes: 15,
		groupBy: ''
	},
	{
		id: 'layout-shift',
		label: 'Layout shift',
		ruleName: 'Layout shift (CLS)',
		metricName: 'browser.web_vital.cls',
		aggregation: 'avg',
		operator: 'gt',
		thresholdValue: 0.25,
		lookbackMinutes: 15,
		groupBy: ''
	},
	{
		id: 'slow-route',
		label: 'Slow route change',
		ruleName: 'Slow route change',
		metricName: 'browser.route_timing.duration',
		aggregation: 'p95',
		operator: 'gt',
		thresholdValue: 3000,
		lookbackMinutes: 15,
		groupBy: 'url.path'
	},
	{
		id: 'frozen-page',
		label: 'Frozen page',
		ruleName: 'Page froze for a second or more',
		metricName: 'browser.long_task.duration',
		aggregation: 'max',
		operator: 'gte',
		thresholdValue: 1000,
		lookbackMinutes: 5,
		groupBy: 'url.path'
	}
];

export const BROWSER_METRIC_NAMES = [
	'browser.web_vital.lcp',
	'browser.web_vital.inp',
	'browser.web_vital.cls',
	'browser.web_vital.fcp',
	'browser.web_vital.ttfb',
	'browser.route_timing.duration',
	'browser.long_task.duration',
	'browser.page_load.duration',
	'browser.page_load.dom_content_loaded'
];
