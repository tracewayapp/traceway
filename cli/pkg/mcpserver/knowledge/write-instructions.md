## Write access

This server was started with write access, so it also has mutating tools: archive_exceptions, unarchive_exceptions, acknowledge_page, resolve_page, set_notification_rule_enabled, snooze_notification_rule, run_check, create_post_mortem, update_post_mortem, set_slow_endpoint, create_sdk_config_rule, update_sdk_config_rule, delete_sdk_config_rule and reorder_sdk_config_rules. Every one of them changes what the whole team sees.

- Only call a mutating tool when the user asked for that change. Investigating, debugging or summarizing never implies a write.
- When a write would cover several records and they differ in a way the user may not know (one is still occurring, has a different cause, or belongs to another team), list them, say how they differ, and ask which ones to change before calling the tool. Do not settle it yourself, even by leaving the odd one out.
- Before the call, state the exact change in one line: the tool, the record and the new value. Then make it.
- Prefer the reversible option: snooze a noisy rule rather than disabling it, acknowledge a page before resolving it, archive rather than resolve with archive_issue.
- After the call, report what changed and how to undo it.
- A 403 is a policy decision; do not retry or look for another route. Tell the user what it said and who can change it. "This credential is read-only" means the client is not connected with a write credential. "Agent write access is turned off" means an organization admin has to turn it on in Settings > Agent Access. "Your role in this project is read-only" means the credential can write but the person's project role is readonly: another credential will not help, and an organization owner or admin can change the role in Settings > Team Members.
- Every agent write is recorded in the organization's agent activity log with the credential that made it (the token, or the OAuth client).

Read traceway://knowledge/operations before your first write: it covers the rules, what each write does, and playbooks for pages, alerts, monitors and post-mortems. For how each area of the product works, read traceway://knowledge/learn.

For OpAMP changes, first list_sdk_config_rules and read the OpAMP playbook in traceway://knowledge/operations. Preserve unrelated settings and triggers, use expected_updated_at for edits, explain sampling and masking, and verify saved settings with preview_sdk_config. This configures existing projects; never provision another project or request an ingest token just to change collection.
