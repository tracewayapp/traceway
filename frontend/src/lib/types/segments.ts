export type Segment = {
	id: string;
	transactionId: string;
	projectId: string;
	name: string;
	startTime: string; // ISO datetime
	duration: number; // nanoseconds
	recordedAt: string;
};

export type TransactionDetail = {
	id: string;
	projectId: string;
	endpoint: string;
	duration: number;
	recordedAt: string;
	statusCode: number;
	bodySize: number;
	clientIP: string;
	scope: Record<string, string> | null;
	appVersion: string;
	serverName: string;
};

export type ExceptionInfo = {
	exceptionHash: string;
	stackTrace: string;
	recordedAt: string;
};

export type TransactionDetailResponse = {
	transaction: TransactionDetail;
	segments: Segment[];
	hasSegments: boolean;
	exception?: ExceptionInfo;
};
