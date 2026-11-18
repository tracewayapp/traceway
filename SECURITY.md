# Security Policy

## Supported Versions

Security fixes land on `main` and ship in the next release. Only the latest release of each component is supported:

| Component                                         | Supported      |
| ------------------------------------------------- | -------------- |
| Traceway server (Docker images, binaries, Helm)   | Latest release |
| `traceway` CLI                                    | Latest release |
| Agent skills (`skills/`, `.claude-plugin/`)       | `main`         |

Older versions do not receive backports. Upgrade to the latest release to get a fix.

## Reporting a Vulnerability

Please do not open a public issue for a security problem.

Email **dusan@tracewayapp.com** with:

- the affected component and version
- steps to reproduce, or a proof of concept
- the impact you expect

You will get an acknowledgement within 3 business days. We will keep you updated while we work on a fix and credit you in the release notes unless you ask us not to.

## Scope

In scope: the backend API, the dashboard, the CLI, the OAuth and device flows, ingest endpoints, and the agent skills in this repository.

Out of scope: the placeholder credentials in `docker-compose*.yml`, `examples/`, `testing/`, `benchmarks/` and test files. They exist for local development and must be replaced in production, as the docs say.
