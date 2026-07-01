# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.8.0] - 2026-07-01

### Added
- Assignee board dispatch, avatar overflow threshold (show all when ≤ 3 assignees, otherwise 2 + an overflow chip), backlog sprint sections, Daily Scrum polish.
- Dedicated sprint planning view at its own URL (`/planning`) — a scrum board with no active sprint no longer shows a placeholder; visiting planning always shows the real view, with its own "Cancel" flow.
- Redesigned backlog: flat priority-ordered list with drag-to-reorder, collapsible active/planning sprint sections above it, and an amber unplanned-work warning when a ticket is dropped onto the active sprint.

### Changed
- Ticket detail page and quick-view drawer now share the `ticket-links` and `ticket-tags` partials instead of duplicating markup.
- Priority, points, and track changes made from the drawer/permalink now reliably refresh the board behind it (client-side `boardUpdated` dispatch instead of relying on a server `HX-Trigger` header, which is lost when the triggering element is detached by an `outerHTML` swap).

### Fixed
- Markdown rendering (headings, lists, blockquotes, GFM tables, inline code) had no typographic styling in ticket descriptions and comments — `@tailwindcss/typography`'s `prose` class was configured in `tailwind.config.js` but never actually applied in any template, a regression from the v0.7.0 layout migration. Restored on all four rendered-markdown surfaces (ticket description, its standalone edit-fragment endpoint, the refinement view, and comments).
- The dual-mode Visual/Code rich markdown editor (originally shipped in v0.5.0) was silently dropped by the same v0.7.0 layout migration — every description and comment field had regressed to a bare `<textarea>` with no formatting toolbar and no `@mention`/`#ticket` autocomplete. Restored and restyled to the current design system; wired back into the ticket description editor (drawer + permalink), comment create/edit, and the New Ticket modal.
- Released Docker images (`ghcr.io/allmend/docket`) were missing `htmx.min.js` and `alpine.min.js` since v0.7.0, shipping a non-interactive UI — the vendored libraries were never wired into the CI/release asset pipeline (`static/dist/` is gitignored, so a fresh CI checkout never had them). `make assets` now runs `scripts/vendor.js` first; the script now fails the build loudly on a failed download instead of continuing silently.
- Removed confirmed-dead code (verified with `golang.org/x/tools/cmd/deadcode`): a superseded login path, unused query methods, and a leftover DaisyUI helper from before the pure-Tailwind migration.

## [0.7.0] - 2026-05-22
### Added
- Retrospective card stacking — drag a duplicate card onto another to group it; stacks expand inline with an unstack action, one level of nesting.
- Backlog sprint section — planning and active sprint tickets shown directly above the backlog list.

### Changed
- Board filter polish and Acceptance Criteria visual separation from the ticket description.

### Fixed
- Backlog avatar stack rendering — solid background color, ring now matches the row background instead of the page background.

## [0.6.0] - 2026-05-21
### Added
- Board filters: severity, assignee, and age.

### Fixed
- A SortableJS `_onDragOver` crash and a stale "closed" chip left behind after dragging a ticket into the Done column.

## [0.5.0] - 2026-05-20
### Added
- Dual-mode rich editor (Visual / Code) for ticket descriptions, with inline markdown shortcuts.
- vim-style `j`/`k` navigation in the Refinement view.

### Fixed
- Arrow-key handling inside textareas.
- Sidebar chevron toggle and a flash-of-unrotated-icon on page load.
- Overlapping avatar stack on board cards — capped at 2 avatars plus a grey overflow chip.
- Login no longer shows an org field — the UI is single-tenant even though the underlying schema stays multi-tenant.

## [0.4.0] - 2026-04-27
### Added
- Daily Scrum view.
- CI/CD pipeline and Kubernetes deployment manifests.

### Changed
- Alpha version bump; container builds restricted to `amd64` (arm64/QEMU dropped for now); provenance attestation disabled to fix a broken `unknown/unknown` image manifest.

## [0.3.0-alpha] - 2026-04-26
### Added
- Sprint capacity planning — per-member focus percentage, seeded from the team, shown in the backlog sidebar and planning board view.
- Definition of Done — board-level checklist with per-ticket check state.
- Acceptance Criteria — markdown field with interactive GFM task-list checkboxes that persist to the database.
- Roadmap view — collapsible sprint rows with progress bars and ticket lists.
- Refinement view — side-by-side backlog and ticket detail at its own URL, with arrow-key navigation and a readiness indicator (green dot once a ticket has both story points and acceptance criteria).

## [0.2.0-alpha] - 2026-04-24
### Added
- Sprint Review and Retrospective board.
- Story points, tags, ticket links (blocks / depends on / duplicates), and notifications.

## [0.1.0-alpha] - 2026-04-23
### Added
- Initial public release: teams, Scrum and Kanban boards, sprints, backlog, tickets, and the core Scrum workflow.

[Unreleased]: https://github.com/allmend/docket/compare/v0.8.0...HEAD
[0.8.0]: https://github.com/allmend/docket/compare/v0.7.0...v0.8.0
[0.7.0]: https://github.com/allmend/docket/compare/v0.6.0...v0.7.0
[0.6.0]: https://github.com/allmend/docket/compare/v0.5.0...v0.6.0
[0.5.0]: https://github.com/allmend/docket/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/allmend/docket/compare/v0.3.0-alpha...v0.4.0
[0.3.0-alpha]: https://github.com/allmend/docket/compare/v0.2.0-alpha...v0.3.0-alpha
[0.2.0-alpha]: https://github.com/allmend/docket/compare/v0.1.0-alpha...v0.2.0-alpha
[0.1.0-alpha]: https://github.com/allmend/docket/releases/tag/v0.1.0-alpha
