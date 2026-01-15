export type ExceptionGroup = {
    exceptionHash: string;
    stackTrace: string;
    lastSeen: string;
    firstSeen: string;
    count: number;
};

export type ExceptionOccurrence = {
    transactionId: string | null;
    transactionType: 'endpoint' | 'task';
    exceptionHash: string;
    stackTrace: string;
    recordedAt: string;
    scope: Record<string, string> | null;
    appVersion: string;
    serverName: string;
    isMessage: boolean;
    endpoint: string;
};

export type LinkedTransaction = {
    id: string;
    endpoint: string;
    duration: number;
    statusCode: number;
    recordedAt: string;
    transactionType: 'endpoint' | 'task';
};
