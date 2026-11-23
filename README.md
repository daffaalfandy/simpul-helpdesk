# SimDesk

**SimDesk** (Simpul Helpdesk) is an open-source, self-hosted helpdesk for IT vendors who support client organisations. One install holds one or more vendor organizations, each with its own clients, projects, tickets and settings.

## Features

**For your clients**
- A portal to report incidents and request services from your catalogue, follow every ticket and reply to the team
- Fixes go to the client for approval (UAT) in the portal or straight from the email; a ticket can be reopened for 3 working days, then it closes by itself
- Requesters see only the projects they are linked to, and can add colleagues to a ticket's "People notified"

**For your team**
- A kanban board with drag-and-drop status changes, filters, search, a saved project filter and a reported-date range
- Replies and internal notes with a formatting toolbar; internal notes never reach clients, in the app or by email
- Attachments, and an audit log of every state change

**SLA you can defend**
- Targets per project and priority, in minutes, hours, working days or calendar days
- Business hours with holidays (Indonesian official holiday lists included), or a 24/7 clock per project
- Clocks pause while the team waits on the client, and optionally while a fix is in UAT
- At-risk and breach alerts, and a Definition of Done per project that guards resolving a ticket
- SLA reports per project over a range of months, with an Excel file for every project and a PDF for each one

**Connected to your code**
- GitHub and GitLab repository webhooks, each with its own secret: a commit that names a ticket key shows on the ticket with the branches it reached
- A Definition of Done item can wait for the fix to reach a branch, and Git ticks it; Git never moves a ticket

**Notifications**
- A bell in the app and emails, switched on or off per event by each person
- Emails right away, hourly or daily; urgent items (P1, SLA, UAT) always go right away

**AI clients (MCP)**
- Staff connect Claude Code or another MCP client with a personal access token to search and read tickets, reply, add internal notes and change status, assignee or priority

**Easy to host**
- One binary with the web app built in, plus PostgreSQL
- Fonts and assets are self-hosted, so an install works without outside services

## Run SimDesk

You need PostgreSQL 18 and an SMTP server for emails (invites, password resets, notifications). Building needs Go 1.26+ (the go command downloads it automatically) and Node 22+.

**1. Build** the binary, with the web app inside it:

```sh
make build        # writes bin/simdesk
```

**2. Configure** with environment variables (every setting is documented in [`.env.example`](.env.example)). For production:

| Variable | Value |
|---|---|
| `APP_ENV` | `prod` |
| `APP_URL` | The address people open, e.g. `https://helpdesk.example.com` (used in email links) |
| `HTTP_ADDR` | Where SimDesk listens, e.g. `127.0.0.1:8080` behind a proxy |
| `DATABASE_URL` | `postgres://user:password@localhost:5432/simdesk?sslmode=disable` |
| `STORAGE_DIR` | An absolute path for uploaded attachments |
| `ENCRYPTION_KEY` | 32 random bytes, base64: `openssl rand -base64 32` |
| `SMTP_ADDR`, `SMTP_USER`, `SMTP_PASS`, `MAIL_FROM` | Your mail server and sender address |
| `TRUSTED_PROXIES` | The address of your HTTPS proxy or tunnel, e.g. `127.0.0.1/32` |

Production serves cookies over HTTPS only, so put SimDesk behind an HTTPS proxy or a tunnel (Caddy, nginx, Cloudflare Tunnel). Keep `ENCRYPTION_KEY` and `STORAGE_DIR` with your database backups: without the key, stored webhook secrets can't be read.

**3. Create the database schema and the first administrator:**

```sh
simdesk migrate up
simdesk user create --role admin --email you@example.com --name "Your Name"
```

Creating the administrator from the command line closes first-run setup, so do it before the address is public. Without it, the first person to open the address goes through first-run setup and becomes the administrator. The administrator creates the first organization after signing in.

**4. Start** the web server and the background jobs (SLA clocks, emails, auto-close):

```sh
simdesk serve --worker
```

To load a year's Indonesian holidays into the business-hours calendar: `simdesk holidays load-preset --year 2027`.

**To update:** build again, replace the binary, run `simdesk migrate up` and restart `simdesk serve --worker`.

## Development

Requirements: Go 1.26+, Node 22+, Docker.

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

`make test` runs all tests (Docker is needed for the database tests), `make lint` the static checks. `make help` lists every target.

## Connect an AI client (MCP)

In SimDesk, open your menu › **MCP access**, make a personal access token (read only, or read and write), and add the server to your client, for example Claude Code:

```sh
claude mcp add --transport http simdesk https://helpdesk.example.com/api/mcp \
  --header "Authorization: Bearer simdesk_pat_…"
```

A token acts as you in the organization you made it in, until it expires or you revoke it.

## Stack

Go (chi, pgx, sqlc, River) · PostgreSQL 18 · React + TypeScript (Vite, TanStack, Tailwind) · OpenAPI 3

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) and the [Code of Conduct](CODE_OF_CONDUCT.md). Report security problems privately, as [SECURITY.md](SECURITY.md) describes. Changes per release are in [CHANGELOG.md](CHANGELOG.md).

## License

[AGPL-3.0](LICENSE). If you run a modified version as a service, you must share your changes.
