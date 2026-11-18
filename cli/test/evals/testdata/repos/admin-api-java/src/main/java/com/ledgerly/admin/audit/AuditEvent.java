package com.ledgerly.admin.audit;

import java.time.OffsetDateTime;

public record AuditEvent(
        long id,
        long actorId,
        String action,
        String targetType,
        String targetId,
        String ipAddress,
        String userAgent,
        OffsetDateTime createdAt) {
}
