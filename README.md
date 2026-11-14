# SimDesk

**SimDesk** (Simpul Helpdesk) is an open-source, self-hosted helpdesk for IT vendors who support client organisations.

- Client portal for reporting issues and approving fixes (UAT)
- Agent board with SLA timers (business hours, pauses while waiting on the client)
- Definition of Done and SLA targets per project, with Done driven by GitHub / GitLab events
- Notifications in the app and by email, chosen per event by each user
- Monthly SLA reports per client

> **Status:** early development. Done: sign-in, users and roles; clients and the business-hours calendar; tickets with replies, internal notes and attachments; projects with their own SLA targets and global priority names. Next: the kanban board.

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
`STORAGE_DIR` (an absolute path; back it up with the database). To create the first admin on a
fresh install:

```sh
simdesk migrate
simdesk user create --role admin --email you@example.com --name "Your Name"
```

## License

[AGPL-3.0](LICENSE). If you run a modified version as a service, you must share your changes.
