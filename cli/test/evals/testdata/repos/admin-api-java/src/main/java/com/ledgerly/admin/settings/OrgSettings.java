package com.ledgerly.admin.settings;

import java.time.OffsetDateTime;

public record OrgSettings(String locale, String timezone, String invoicePrefix, OffsetDateTime updatedAt) {
}
