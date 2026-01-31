export type ExceptionGroup = {
    exceptionHash: string;
    stackTrace: string;
    lastSeen: string;
    firstSeen: string;
    count: number;
};

export type ExceptionOccurrence = {
    id: string;
    traceId: string | null;
    traceType: 'endpoint' | 'task';
    exceptionHash: string;
    stackTrace: string;
    recordedAt: string;
    scope: Record<string, string> | null;
    appVersion: string;
    serverName: string;
    isMessage: boolean;
    endpoint: string;
};

export type LinkedTrace = {
    id: string;
    endpoint: string;
    duration: number;
    statusCode: number;
    recordedAt: string;
    traceType: 'endpoint' | 'task';
};
