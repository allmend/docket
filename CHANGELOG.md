# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.12.0] - 2026-07-22

### Added
- Ticket activity is a single chronological stream: history entries and comments interleaved oldest-first, instead of every field change above every comment.
- Closed tickets are immutable server-side (409 Conflict) — title, body, acceptance criteria, priority, points, assignees, labels, comments, links and DoD checks. Moving a closed ticket stays allowed, since dragging it out of Done is how you reopen it.
- Daily Scrum: `j` / `k` move between people, mirroring Backlog Refinement.
- Relative timestamps ("22m ago") carry the exact ISO 8601 time as a native hover tooltip.
- Breadcrumbs for the board, backlog and planning pages; Daily Scrum nests under Board, matching its URL.
- Build-time version stamping via `internal/version` — `make build` derives it from `git describe`, release images from the tag.
- Template/handler data mismatches now fail the build: `missingkey=error` plus a static test that derives each template's required fields and each handler's supplied keys and diffs them, including the `dict`-based partial calls.

### Changed
- Sidebar workspace order is now Dashboard, Board, Backlog.
- The comment composer is taller and separated from the activity stream by a divider.

### Fixed
- Priority sorted alphabetically, so medium ranked above critical on My Issues and all dashboard lists.
- `/metrics` reported 0 committed and completed points for in-flight sprints, leaving burndown flat until the sprint closed.
- The board filter's Tracks section always listed nothing — `BoardTags` was never passed to the template.
- Sprint review's "create retro" link never rendered, and the sign-in page never showed the organisation name.
- The sign-in page's show-password toggle did nothing: the auth layout never loaded Alpine. The version shown there read `v0.1.0`.
- The blocked badge disappeared when a ticket moved into a planning-sprint column.
- Ticket activity did not refresh after a status change.
- Selected rows shifted ~2px sideways in Refinement, Daily Scrum and the Inbox.
- Filtering the backlog rendered every ticket twice in two different styles.
- Filter inputs drew a stray border from the forms plugin.
- An overdue sprint read "-17 days left" instead of "17 days overdue".

## [0.11.0] - 2026-07-06

### Added
- Workspace settings rebuilt to the design handoff: General (rename + description), Members (role chips, email, filter), Tracks, Sprint defaults, and Danger zone tabs.
- Track management: create, edit, and delete tracks from workspace settings — each with a color from the five-hue palette, a description, and an optional lead. The table shows open ticket counts and open points per track.
- Team-settable sprint capacity (workspace settings → Sprint defaults). The Sprint Planning bar shows committed points against it, live-updating during drag and turning amber when over capacity.
- Sprint duration picker on the planning bar: start date, 1w/2w/3w presets, and a free end date in one popover, replacing the fixed preset buttons.
- Per-user avatar colors derived from the name — same color everywhere a user appears, including the board filter's assignee chips.
- The Sprint Board filter now includes Tracks.
- Ticket IDs are links everywhere (board cards, backlog, daily scrum, drawer): open tickets in new tabs with cmd-click; clicking elsewhere on a card still opens the drawer.
- Closing a ticket automatically unblocks the tickets it was blocking, with the cleared links recorded in both tickets' activity. All link changes are now logged on both ends.
- @/# autocomplete appears at the caret instead of under the field, in both editor modes.

### Changed
- Title and description edit separately: click the title for an inline rename, click the description for the rich editor — which now opens at the rendered content's height.
- Escape steps back one level at a time in the ticket drawer (popup → input → inline edit → drawer) instead of closing it outright.
- Popups behave native: opening one closes others, they stack above all content, and they never leave the viewport.
- The New Ticket modal is larger, and no longer shows sprint details — new tickets always start in the backlog.
- Linked work matches the criteria pattern: an "+ Add link" row at the bottom of the list, shown even when the list is empty.
- The backlog's "In sprint" section starts collapsed.
- The sidebar sprint overview (day / burned / remaining) updates live as tickets move.
- Editor toolbar icons replaced with Heroicons.
- Blocked badges read "Blocked by ENG-16" instead of "Blocked · ENG-16".

### Fixed
- The assignee and tracks "add" popups opened behind or outside the drawer — or not at all. Three separate root causes fixed (overflow clipping, an Alpine style-binding conflict, and a handler syntax error Alpine swallowed silently).
- @/# autocomplete no longer leaves a phantom empty box behind, reopens after picking a suggestion, or re-triggers on Enter from the previous line.
- Acceptance criteria: Enter saves and chains straight into the next criterion; Escape cancels the input without closing the drawer.
- Dropping a ticket into a drag-drop gap that had become too small could lock up the server in an endless rebalance loop. The column now renumbers once and the ticket lands in the intended slot.

### Removed
- The short-lived `++underline++` markdown extension — Docket renders pure CommonMark/GFM only. `_text_` remains italic, per the spec.

## [0.10.0] - 2026-07-03

### Added
- Code blocks in tickets and comments now render as panels with a header bar — a colored language dot (green for shell, blue for JSON, emerald otherwise) and the language label — with syntax highlighting unchanged inside. The editor's visual pane shows the same treatment.
- Quote and link buttons in the editor toolbar, working in both Visual and Code modes.
- The formatting toolbar now works in Code mode too: buttons insert (and toggle) the markdown syntax directly in the textarea — bold/italic/strike/inline-code wrapping, heading prefixes, list and quote line prefixes, code fences, links.
- `CONTRIBUTING.md` — dev setup, PR checklist, conventions, and the contributor license agreement (previously referenced but missing).

### Changed
- The visual editor was rebuilt on native browser editing: typing, Enter, and undo now behave like any text field, fixing erratic cursor jumps and invisible-caret issues when pressing Enter. The block-type dropdown is replaced by discrete H1/H2/H3 buttons, and the mode switch is a segmented Visual/Code toggle.
- The editor pane keeps its height when switching between Visual and Code modes.
- Lists converted from the visual pane now use tight `- item` / `1. item` markers instead of column-padded ones.
- `COMMERCIAL.md` rewritten to accurately describe what the AGPL-3.0 permits and obliges: third-party hosting is legal but requires full source disclosure to the service's users and may not use the Docket name; commercial licenses are the escape hatch from copyleft, not a hosting permit.
- README: added full-text search and user management to features, `MODE` to the env-var table, contributing section, and copyright notice.

### Fixed
- Refinement: the selected ticket survives list re-renders (points/priority/AC edits no longer reset the selection), and the selection is written to the URL hash so a refresh resumes where you left off.
- Board cards re-render when dragged into or out of a Done column (strikethrough and checkmark update immediately).
- The refinement detail pane offers the full description editor and ticket links, matching the drawer and permalink page.

## [0.9.0] - 2026-07-02

### Security
- API tokens with `metrics:read` or `api:read` scope could perform writes through the internal HTMX endpoints, which accepted any valid API token regardless of scope. The UI surface now requires `api:write` scope; browser sessions are unaffected.
- Ticket drag-and-drop now verifies the target column belongs to the caller's org, and assigning a label to a ticket now verifies the label's org — closing two cross-tenant integrity gaps ahead of multi-org deployments.
- Label colors are validated as strict `#RRGGBB` hex values at creation time.
- Member creation failures no longer echo internal database error details to the client.
- Logout now clears session cookies with the same HttpOnly/Secure/SameSite attributes they were set with, and `/static/` no longer serves directory listings.

### Added
- Store-layer integration tests that run against a real PostgreSQL (via testcontainers). `make test-short` skips them for a fast inner loop without Docker.
- Tagged releases now automatically publish a GitHub Release with notes extracted from this changelog, alongside the container image push to `ghcr.io/allmend/docket`.

### Changed
- API handler cleanup: shared `parseForm` and `ticketFromPath` helpers replace repeated parse/lookup/error boilerplate across the ticket handlers.
- Priority color-bar and label rendering moved from long per-template conditionals into `priorityColor`/`priorityLabel` template functions (the emitted classes are safelisted in the Tailwind config).
- Docker build context slimmed with a `.dockerignore` (excludes `.git`, `node_modules`, scratch dirs) — faster image builds, no change to image contents.
- Dependency updates: pgx 5.9.2, golang.org/x/crypto 0.51.0, golang.org/x/net 0.53.0; markdown/highlighting libraries now correctly declared as direct dependencies.

### Removed
- Leftover sqlc scaffolding (`sqlc.yaml`, `sql/queries/`, `make sqlc`) — the store has always used pgx directly; the config was never wired up and its queries referenced tables from before the projects→teams rename.
- `cmd/seed` and `make seed` — superseded by the idempotent auto-seed that runs on every startup with the same `SEED_*` environment variables.
- Valkey/Redis from all compose files, `REDIS_URL` from the config — nothing consumes it; it was plumbing for a queue that Docket never grew. It can return with the WebSocket fan-out phase.
- `daisyui` from devDependencies — dropped from the Tailwind config during the pure-Tailwind migration, the package itself was never removed.

### Fixed
- The self-hosting `deploy/docker-compose.yml` declared a `wget`-based container healthcheck, but the image is distroless (no shell, no wget), so the container could never report healthy. Removed in favour of external probing; the Kubernetes manifests were unaffected (kubelet-driven `httpGet` probes).

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

[Unreleased]: https://github.com/allmend/docket/compare/v0.12.0...HEAD
[0.12.0]: https://github.com/allmend/docket/compare/v0.11.0...v0.12.0
[0.11.0]: https://github.com/allmend/docket/compare/v0.10.0...v0.11.0
[0.10.0]: https://github.com/allmend/docket/compare/v0.9.0...v0.10.0
[0.9.0]: https://github.com/allmend/docket/compare/v0.8.0...v0.9.0
[0.8.0]: https://github.com/allmend/docket/compare/v0.7.0...v0.8.0
[0.7.0]: https://github.com/allmend/docket/compare/v0.6.0...v0.7.0
[0.6.0]: https://github.com/allmend/docket/compare/v0.5.0...v0.6.0
[0.5.0]: https://github.com/allmend/docket/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/allmend/docket/compare/v0.3.0-alpha...v0.4.0
[0.3.0-alpha]: https://github.com/allmend/docket/compare/v0.2.0-alpha...v0.3.0-alpha
[0.2.0-alpha]: https://github.com/allmend/docket/compare/v0.1.0-alpha...v0.2.0-alpha
[0.1.0-alpha]: https://github.com/allmend/docket/releases/tag/v0.1.0-alpha
