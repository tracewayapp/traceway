export const ruleTypeOptions = [
	{ value: 'new_error', label: 'New Issue' },
	{ value: 'impact_score_critical', label: 'Impact Score Critical' },
	{ value: 'impact_score_high', label: 'Impact Score High' },
	{ value: 'impact_score_medium', label: 'Impact Score Medium' },
	{ value: 'error_regression', label: 'Error Regression' },
	{ value: 'error_rate_threshold', label: 'Error Rate' },
	{ value: 'error_count_threshold', label: 'Error Count' },
	{ value: 'endpoint_p95_threshold', label: 'Endpoint P95' },
	{ value: 'endpoint_p99_threshold', label: 'Endpoint P99' },
	{ value: 'endpoint_error_rate', label: 'Endpoint Error Rate' },
	{ value: 'apdex_drop', label: 'Apdex Drop' },
	{ value: 'metric_threshold', label: 'Metric Threshold' },
	{ value: 'no_data', label: 'No Data' },
	{ value: 'task_duration_threshold', label: 'Task Duration' },
	{ value: 'task_failure_rate', label: 'Task Failure Rate' },
	{ value: 'throughput_drop', label: 'Throughput Drop' },
	{ value: 'ai_trace_cost', label: 'AI Trace Cost' }
];

export const ruleTypeLabels: Record<string, string> = Object.fromEntries(
	ruleTypeOptions.map((o) => [o.value, o.label])
);
