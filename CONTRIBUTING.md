# Contributing to SimDesk

Bug reports, fixes and features are welcome.

## Before you start

- For anything bigger than a small fix, open an issue first so we can agree on the approach.
- Security problems go through [SECURITY.md](SECURITY.md), not the issue tracker.
- Everyone taking part follows the [Code of Conduct](CODE_OF_CONDUCT.md).

## Set up

Follow [Development](README.md#development) in the README: Go 1.26+, Node 22+ and Docker, then `cp .env.example .env`, `make up`, `make migrate`, `make seed` and `make dev`.

## Rules the code follows

- **API first.** API changes start in `api/openapi.yaml`; run `make gen` to regenerate the Go server and the TypeScript types. SQL changes go in `api/queries/` or `api/migrations/`, then `make gen` (sqlc). Never edit generated files by hand.
- **Organizations.** Every query on organization-owned data takes a required `organization_id`; composite foreign keys block links across organizations.
- **Client data.** Requester data is always filtered by client in the service layer. Internal notes never reach requesters: not through the API, a notification or an email.
- **Audit.** State changes write the audit log in the same transaction.
- **Pure logic.** SLA, Definition of Done and commit-message parsing live in pure packages with table-driven tests.
- **No magic values.** Limits, durations, statuses, roles, error codes and the like are named constants near their domain.
- **UI.** Use the design tokens in `web/src/styles/tokens.css`, never raw hex colours or sizes. Use the Dropdown component, not a native `<select>`. Every UI string goes in `web/src/i18n/en.json`, and the copy says "client", not "agency".
- **Offline installs.** No runtime CDNs: fonts and assets are self-hosted.

## Checks

There is no CI. Before you open a pull request, run:

```sh
make gen-check   # generated code is up to date
make lint        # gofmt, go vet, staticcheck, ESLint, tsc, no raw hex
make test        # Go tests (PostgreSQL in Docker) and Vitest
```

## Commits and pull requests

- Commit messages follow [Conventional Commits](https://www.conventionalcommits.org/): `feat:`, `fix:`, `docs:`, `test:`, `chore:`, with a scope when it helps (`fix(web): …`).
- Keep each pull request to one change, with its tests, and add a line under Unreleased in [CHANGELOG.md](CHANGELOG.md) for anything users will notice.

## License

By contributing, you agree that your contributions are licensed under the [AGPL-3.0](LICENSE), like the rest of SimDesk.
