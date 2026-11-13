<!-- GENERATED FILE: copied from cli/pkg/mcpserver/knowledge/personal-notifications.md by cli/tools/skillgen. Edit there and run just gen-skills in cli/. -->
# Personal contacts and notification chains

A page's escalation policy selects a person. That person's **contact methods** hold delivery destinations; their **high** and **low** urgency chains decide when each destination is tried. These settings belong only to the authenticated person, even if they are an organization admin. They are not project alert rules or team-wide channels.

Select a project whose organization permits agent writes; this supplies permission and audit context for personal mutations. `contact-methods list` and `notification-chains show` inspect the current user's settings. Never assume you can configure another responder through your credential.

## Contact methods

```json
{"methodType":"email","config":{"email":""},"enabled":true}
```

An empty email uses the account email. Other method types/configuration:

| `methodType` | `config` |
|---|---|
| `slack` | `webhookUrl`, optional `channel`, `username` |
| `pushover` | `userKey`, `appToken` |
| `telegram` | `botToken`, `chatId` |
| `sms` | `phoneNumber` in E.164, e.g. `+12025550123` |

```bash
traceway-write contact-methods validate create --file contact.json
traceway-write contact-methods create --file contact.json --yes
traceway-write contact-methods update 7 --file contact.json --yes
traceway-write contact-methods list
```

Updates can change config and enabled state. Preserve credentials through the environment's secure mechanism; do not copy secrets into reports. Deleting a method uses `contact-methods delete <id>`. Testing (`contact-methods test <id>`) sends a real notification and needs a request to test delivery.

SMS requires server Twilio configuration. Creating a number or changing it sends a verification code; this is a real outbound SMS. Only verified enabled contacts can appear in a chain. Verify with `contact-methods verify <id> --file code.json --yes`, where `code.json` is `{"code":"123456"}` supplied by the user. Codes expire in 10 minutes with at most 5 attempts. `contact-methods resend-code <id>` sends another code, limited to 3 per hour. Do not guess codes. A changed phone number must be verified again.

MCP: `list_contact_methods`, `create_contact_method`, `update_contact_method`, `delete_contact_method`, `test_contact_method`, `verify_contact_method`, `resend_contact_method_code`. Use `id` and `data` as applicable, with `project_id` for the write context.

## Notification chains

```json
{
  "high":[{"contactMethodId":7,"delayMinutes":0},{"contactMethodId":9,"delayMinutes":5}],
  "low":[{"contactMethodId":7,"delayMinutes":0}]
}
```

```bash
traceway-write notification-chains show
traceway-write notification-chains validate replace --file chains.json
traceway-write notification-chains replace --file chains.json --yes
traceway-write notification-chains show
```

`replace` replaces **both** chains atomically. Read both first and preserve the one not being changed. Each has up to 10 steps with delays from 0–120 minutes. Delays are measured from when the page reaches the person, not added cumulatively; array order determines chain ordering. Referenced methods must belong to this user and be enabled and verified. MCP equivalents are `get_user_notification_rules` and `set_user_notification_rules` (`data` contains both arrays).

The page's urgency chooses the chain. Acknowledging or resolving the page cancels later pending personal deliveries. If the first method becomes unusable, remaining steps move earlier so the first usable destination is immediate. No configured chain means all enabled verified methods at once; no usable enabled, verified and sendable methods means account email. Emptying chains does not silence pages. To pause a source, snooze its project alert rule; to take responsibility, acknowledge the page.
