# Changelog

All notable changes to SimDesk are listed here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and versions follow [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added

- Contributing guide, code of conduct, security policy and this changelog.

## [1.0.0] - 2026-11-23

First release.

### Added

- Several vendor organizations per install, each with its own clients, projects, tickets and settings; roles per organization and administrators across the whole install. The first administrator comes from first-run setup or `simdesk user create`.
- Client portal: report incidents and service requests from the catalogue, follow and reply to tickets, approve fixes (UAT) in the portal or from the email, and reopen within 3 working days before a ticket closes by itself. Requesters see only the projects they are linked to and can add colleagues under "People notified".
- Agent board with drag-and-drop status changes, filters, search, a saved project filter and a reported-date range. Replies and internal notes with a formatting toolbar (stored as Markdown), attachments, and an audit log of every state change.
- SLA per project and priority, in minutes, hours, working days or calendar days. Business hours with holidays (Indonesian official lists for 2026 and 2027) or a 24/7 clock; clocks pause while waiting on the client and optionally during UAT; at-risk and breach alerts.
- Definition of Done per project that guards resolving a ticket, or serves as guidance only, with an optional "code reached a branch" item that Git ticks.
- GitHub and GitLab repository webhooks with a secret per repository: a commit that names a ticket key shows on the ticket with the branches it reached. Git never changes a ticket's status.
- Notifications in the app and by email, switched per event by each person, sent right away, hourly or daily; urgent items (P1, SLA, UAT) always go right away.
- SLA reports per project over a range of months, with an Excel file and a PDF per project.
- MCP server for AI clients, with personal access tokens (read only, or read and write).
- Settings: organization name, logo, ticket prefix and time zone; members and roles with emailed invites; requesters invited on each client.
- One binary with the web app built in, plus PostgreSQL 18. Fonts and assets are self-hosted.

[Unreleased]: https://github.com/daffaalfandy/simpul-helpdesk/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/daffaalfandy/simpul-helpdesk/releases/tag/v1.0.0
