# Security policy

## Supported versions

Security fixes go into the latest release.

| Version | Supported |
|---|---|
| 1.0.x | Yes |

## Report a vulnerability

Please don't open a public issue for a security problem. Report it privately instead:

- [Report a vulnerability on GitHub](https://github.com/daffaalfandy/simpul-helpdesk/security/advisories/new) (preferred), or
- email m.daffaalfandy@gmail.com.

Include what an attacker can do, the steps or a proof of concept, the version or commit, and your setup (reverse proxy, tunnel, mail server) when it matters. You'll get an answer in the advisory or by email. Once a fix is released, the advisory is published and credits you, unless you'd rather not be named.

## Running SimDesk safely

- Run with `APP_ENV=prod` behind HTTPS (a reverse proxy or a tunnel), and set `TRUSTED_PROXIES` to that proxy's address only, so the sign-in limits see real client addresses.
- Generate your own `ENCRYPTION_KEY` and keep it secret. SimDesk refuses the public development key in production.
- Never load the demo data (`simdesk seed`) on an install other people can reach: every demo account shares a password printed in this repository.
- Keep a mail catcher such as Mailpit private or behind a password: it holds password reset and invite links.
