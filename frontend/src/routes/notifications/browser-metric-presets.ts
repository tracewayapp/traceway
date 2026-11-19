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
 * Starting points for metric alerts on a browser project. The web vitals are read at their 75th percentile, the
 * percentile they are defined by, against their "poor" boundary; a freeze is a block of a second or more.
 */
export const BROWSER_METRIC_PRESETS: BrowserMetricPreset[] = [
	{
		id: 'slow-lcp',
		label: 'Slow LCP',
		ruleName: 'Slow page loads (LCP)',
		metricName: 'browser.web_vital.lcp',
		aggregation: 'p75',
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
		aggregation: 'p75',
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
		aggregation: 'p75',
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
		aggregation: 'p75',
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
	},
	{
		id: 'few-good-loads',
		label: 'Few good page loads',
		ruleName: 'Fewer than 75% of page loads have a good LCP',
		metricName: 'browser.web_vital.lcp',
		aggregation: 'good_ratio',
		operator: 'lt',
		thresholdValue: 75,
		lookbackMinutes: 30,
		groupBy: ''
	}
];

/** The same for a mobile project, on the app.* metrics a mobile SDK reports. */
export const MOBILE_METRIC_PRESETS: BrowserMetricPreset[] = [
	{
		id: 'slow-start',
		label: 'Slow app start',
		ruleName: 'Slow app start',
		metricName: 'app.start.duration',
		aggregation: 'p75',
		operator: 'gt',
		thresholdValue: 3000,
		lookbackMinutes: 60,
		groupBy: ''
	},
	{
		id: 'slow-screen',
		label: 'Slow screen',
		ruleName: 'Slow screen display',
		metricName: 'app.screen.ttid',
		aggregation: 'p75',
		operator: 'gt',
		thresholdValue: 1000,
		lookbackMinutes: 60,
		groupBy: 'screen'
	},
	{
		id: 'long-hangs',
		label: 'Long hangs',
		ruleName: 'App hangs of two seconds or more',
		metricName: 'app.hang.duration',
		aggregation: 'p95',
		operator: 'gte',
		thresholdValue: 2000,
		lookbackMinutes: 60,
		groupBy: ''
	}
];

export const MOBILE_METRIC_NAMES = [
	'app.start.duration',
	'app.screen.ttid',
	'app.screen.ttfd',
	'app.hang.duration',
	'app.memory.peak',
	'app.request.duration'
];

export const BROWSER_METRIC_NAMES = [
	'browser.web_vital.lcp',
	'browser.web_vital.inp',
	'browser.web_vital.cls',
	'browser.web_vital.fcp',
	'browser.web_vital.ttfb',
	'browser.route_timing.duration',
	'browser.long_task.duration',
	'browser.long_task.blocking_time',
	'browser.request.duration',
	'browser.page_load.duration',
	'browser.page_load.dom_content_loaded'
];
