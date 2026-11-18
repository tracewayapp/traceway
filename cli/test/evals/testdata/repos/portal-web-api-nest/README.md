# portal web-api

The backend-for-frontend of the customer portal (NestJS). It serves the
portal's pages by calling the internal services:

| Service | What the portal reads |
|---|---|
| accounts-service | account profile, members, feature flags |
| billing-service | open invoices and the current balance |
| notifications-service | unread notifications |
| recommendations-service | suggested catalog items |

Traces and logs (the access log in `src/access-log.ts`) go to Traceway over
OTLP (`src/tracing.ts`, loaded with `node -r`).

```
npm run build && npm start
```
