## Write access

This server was started with write access, so it also has mutating tools: archive_exceptions, unarchive_exceptions, acknowledge_page, resolve_page, set_notification_rule_enabled, snooze_notification_rule, run_check, create_post_mortem, update_post_mortem and set_slow_endpoint. Every one of them changes what the whole team sees.

- Only call a mutating tool when the user asked for that change. Investigating, debugging or summarizing never implies a write.
- Before the call, state the exact change in one line: the tool, the record and the new value. Then make it.
- Prefer the reversible option: snooze a noisy rule rather than disabling it, acknowledge a page before resolving it, archive rather than resolve with archive_issue.
- After the call, report what changed and how to undo it.
- A 403 that says the credential is read-only, or that agent write access is off for the organization, is a policy decision. Do not retry or look for another route. Tell the user what it said; an organization admin turns agent writes on in Settings > Agent Access.
- Every agent write is recorded in the organization's agent activity log with the credential that made it (the token, or the OAuth client).

Read traceway://knowledge/operations before your first write: it covers the rules, what each write does, and playbooks for pages, alerts, monitors and post-mortems. For how each area of the product works, read traceway://knowledge/learn.
