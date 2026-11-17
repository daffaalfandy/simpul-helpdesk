# SimDesk

**SimDesk** (Simpul Helpdesk) is an open-source, self-hosted helpdesk for IT vendors who support client organisations.

- Several vendor organizations on one install, each with its own clients, projects, tickets and settings
- Client portal for reporting issues and approving fixes (UAT)
- Agent board with SLA timers (business hours, pauses while waiting on the client)
- Definition of Done and SLA targets per project
- GitHub / GitLab commits shown on the tickets they name, with the branches they reached
- Notifications in the app and by email, chosen per event by each user
- Monthly SLA reports per client

> **Status:** early development. Done: sign-in, users and roles; clients and the business-hours calendar; tickets with replies, internal notes and attachments; projects with their own SLA targets and global priority names; the kanban board with filters, search and drag-and-drop status changes; notifications in the app and by email (per-event switches, grouped emails and summaries, colleagues under "People notified"); live SLA clocks with pauses, at-risk and breach alerts; a Definition of Done per project (or per ticket, with a reason) that guards resolving, client UAT approval in the portal and from email, and the 3-working-day reopen window with auto-close; the read-only GitHub/GitLab connector (repository webhooks with their own secret; a commit that names a ticket shows on it with the branches it reached, on the board and in notifications, and never moves the ticket); the monthly SLA report per client with an Excel export for every client and a PDF for each; settings for the organisation (name, logo, ticket prefix, time zone), members and roles with emailed invites, requesters invited on each client; several vendor organizations per install (people pick one after signing in and switch from the header, with a role in each; administrators work in all of them), a project filter saved per person for the board and ticket lists; and first-run setup (the administrator, then the first organization). Next: the production image and self-hosting guides.

## Stack

Go (chi, pgx, sqlc, River) · PostgreSQL 18 · React + TypeScript (Vite, TanStack, Tailwind) · OpenAPI 3

## Development

Requirements: Go 1.26+ (the go command downloads it automatically), Node 22+, Docker.

```sh
cp .env.example .env
make up        # Postgres + Mailpit
make migrate   # database schema
make seed      # demo data; prints the dev password
make dev       # API on :8080, web on :5173
```

- App: http://localhost:5173
- Mail catcher: http://localhost:8025
- API docs: http://localhost:8080/api/docs

`make test` runs all tests (Docker is needed for the database tests). `make help` lists every target.

## Self-hosting

A production image and guide are coming. Uploaded attachments live on local disk in
`STORAGE_DIR` (an absolute path; back it up with the database). `ENCRYPTION_KEY` (32 random
bytes, base64: `openssl rand -base64 32`) encrypts secrets kept in the database, such as webhook
secrets; keep it with your backups. The GitHub/GitLab connector works through webhooks, so
GitHub or your GitLab must be able to reach the install's address. On a fresh install, run
`simdesk migrate`, then open the install's address: first-run setup creates the first admin,
once, who then creates the first organization. Or create the first admin from the command line,
which closes setup too; they create the first organization after signing in:

```sh
simdesk migrate
simdesk user create --role admin --email you@example.com --name "Your Name"
```

## License

[AGPL-3.0](LICENSE). If you run a modified version as a service, you must share your changes.
