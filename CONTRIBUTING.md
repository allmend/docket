# Contributing to Docket

Thanks for wanting to contribute! Docket is community-built free software — issues, pull requests, docs fixes, and bug reports are all welcome.

---

## Getting started

**Requirements:** Go 1.25+, Node.js 22+, Docker (for Postgres and the store integration tests).

```bash
# Start dependencies (Postgres + Mailpit)
make docker-up

# Build frontend assets
npm ci
make assets

# Run the app
go run ./cmd/serve
```

On first start a default org and admin user are created (`admin` / `changeme`). See the [README](README.md) for details.

---

## Before you open a pull request

Run the full checklist — CI enforces it, but locally is faster:

```bash
go vet ./...        # zero warnings
go build ./...      # clean compile
make test           # full suite (store tests need Docker)
make test-short     # fast inner loop — skips store integration tests
make css            # rebuild Tailwind if you touched templates
```

Conventions:

- **Branches**: `feat/<short-description>`, `fix/<short-description>`, `chore/<short-description>`, branched off `main`.
- **Commits**: [Conventional Commits](https://www.conventionalcommits.org/) — `feat:`, `fix:`, `chore:`, `docs:`, `refactor:`. Breaking changes use `feat!:` or a `BREAKING CHANGE:` footer.
- **PRs** target `main` and are squash-merged.
- New store methods must filter by `org_id` — no exceptions. New entities get full CRUD in both UI and API.
- Add tests for new pure logic (model methods, helpers, template functions).
- Update `CHANGELOG.md` under `[Unreleased]` for user-visible changes.

Architecture ground rules (server-side rendered, HTMX-first, no ORM, no JS frameworks) are non-negotiable — if a change fights them, open an issue to discuss before writing code.

---

## Contributor License Agreement

Docket is dual-licensed: the community edition is AGPL-3.0 forever, and the project also offers commercial licenses (see [COMMERCIAL.md](COMMERCIAL.md)). To make that legally possible, we ask every contributor to agree to a lightweight CLA before their first pull request is merged.

By submitting a contribution to this repository you agree that:

1. You are the author of the contribution (or otherwise have the right to submit it), and you license it to the Docket maintainers under the AGPL-3.0 **and** grant the maintainers a perpetual, worldwide, non-exclusive, royalty-free right to license the contribution under other terms, including commercial licenses.
2. You retain full copyright to your contribution and remain free to use it for any purpose, under any license, anywhere else.
3. The community edition containing your contribution will always remain available under the AGPL-3.0 — this grant is irrevocable in that direction too.

Signal agreement by adding a `Signed-off-by: Your Name <email>` trailer to your commits (`git commit -s`). A CLA bot may be added later; sign-offs made before that count.

If your employer owns your work, make sure you are allowed to contribute under these terms.

---

## Reporting security issues

Please do **not** open public issues for security vulnerabilities. Report them privately via GitHub's security advisory feature ("Report a vulnerability" on the repo's Security tab).

---

## Questions

Open a GitHub issue or discussion. We are reasonable people.
