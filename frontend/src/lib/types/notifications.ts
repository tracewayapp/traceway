export type NotificationChannelConfig = {
	recipients?: string[];
	url?: string;
	method?: string;
	secret?: string;
	headers?: Record<string, string>;
	webhookUrl?: string;
	channel?: string;
	username?: string;
	token?: string;
	owner?: string;
	repo?: string;
	labels?: string[];
	userKey?: string;
	appToken?: string;
	device?: string;
	priority?: number;
	retry?: number;
	expire?: number;
	callback?: string;
	sound?: string;
	html?: boolean;
	ttl?: number;
	botToken?: string;
	chatId?: string;
	policyId?: number | null;
	integrationId?: number | null;
	profileId?: number | null;
	approval?: string;
};

// Credentials come back from the API as this value; sending it back on an
// update keeps the stored credential.
export const SECRET_SENTINEL = '********';

export type NotificationRuleConfig = {
	thresholdPercent?: number;
	lookbackMinutes?: number;
	minRequests?: number;
	endpoint?: string;
	thresholdMs?: number;
	thresholdApdex?: number;
	metricName?: string;
	operator?: string;
	thresholdValue?: number;
	aggregation?: string;
	tags?: Record<string, string>;
	dataType?: string;
	silenceMinutes?: number;
	thresholdCount?: number;
	taskName?: string;
	minExecutions?: number;
	dropPercent?: number;
	baselineWindowMinutes?: number;
	ignorePatterns?: string[];
	traceName?: string;
	thresholdCost?: number;
	terms?: string[];
};
