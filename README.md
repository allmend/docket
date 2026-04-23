# Docket

> ⚠️ **This is pre-alpha software.** Expect bugs, missing features, breaking changes, and the occasional existential crisis. Not recommended for production use. You have been warned — twice.

Docket is a lightweight, self-hostable Scrum-first ticket and task management tool for engineering teams. An open-source alternative to Linear, Jira, and Shortcut — built close to the Scrum Guide, without the billion settings.

Part of the [Allmend](https://github.com/allmend) suite of open-source tools.

---

## Features (so far)

- Teams, boards, and tickets with display IDs (`ENG-42`)
- Kanban and Scrum board modes
- Sprint lifecycle — planning, active, close
- Product backlog with drag-to-reorder
- Ticket detail with inline editing, Markdown support, and syntax highlighting
- Assignees, priority, status
- Comments with `@mention` linking and autocomplete
- Ticket linking (blocks / relates to)
- Notification inbox
- Ticket and comment history

---

## Tech stack

Go · PostgreSQL · HTMX · Alpine.js · Tailwind CSS · sqlc · Chi

Single binary. No JS framework. Server-side rendered. Fast.

---

## Getting started

**Requirements:** Go 1.22+, PostgreSQL 16+, Node.js (for CSS/JS build)

```bash
# Start dependencies
docker compose up -d

# Install JS dependencies and build frontend assets
npm install
npm run vendor
make css
make js

# Run
go run ./cmd/serve
```

Copy `.env.example` to `.env` and adjust to your environment before running.

On first start, a default org and admin user are created automatically:

```
username: admin
password: admin
```

Change the password immediately.

---

## License

[AGPL-3.0](LICENSE) — self-hosting is free, forever. See [COMMERCIAL.md](COMMERCIAL.md) for commercial use.
