# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- `docs/operations.md`: how startup migration works, an upgrade checklist, rolling back, backups, and recovering a dirty database.
- Settings → About shows the running version, the licence and a link to the exact source release. The login page carries a copyright and source line.
- Docket logs its version and mode at startup (`starting docket`).

### Changed
- CI runs the full test suite, including the Postgres store tests, and fails when they can't run instead of skipping them. `make vet` also fails on unformatted Go code.
- IBM Plex is served by Docket itself instead of Google Fonts: no third-party requests, and the UI looks the same on networks without internet access. The fonts (SIL Open Font License, `static/dist/fonts/OFL.txt`) are cached for a year.
- The example Kubernetes Deployment and Compose file pin an exact image version instead of `latest`.
- Releases start the built image against a throwaway Postgres and check `/readyz`, `/livez` and the login page before publishing it. `make smoke` runs the same check locally.

## [0.18.0] - 2026-10-21

### Changed
- Static assets have content-hashed URLs and are cached by the browser for a year, so moving between pages fetches no CSS or JavaScript. A new release changes the URLs, so nobody runs stale assets after an upgrade.
- The ticket drawer opens without waiting for comments and history; the activity loads right after. Tickets with a long history open as fast as new ones.

### Development
- CI fails when the gzipped static assets exceed the size budget (`make budget`, 84 KiB today), printing each file's size.

## [0.17.0] - 2026-10-21

### Added
- `GET /api/v1/tickets/{ref}` includes the ticket's links (relation and the other ticket's ID, title and state).
- The Backlog page's active-sprint section can change a ticket's points after a warning. The history records it as a re-estimate during the sprint.

### Changed
- Kanban and Blank boards show Done tickets closed in the last 14 days. A footer counts the older ones and links to show all.
- Changing the points of a ticket in the active sprint needs an explicit override, also through the HTMX route; without it the server answers 400. Before, only the UI locked them.
- An omitted field in the v1 `PUT` endpoints keeps its value and an explicit `""` clears it, for a ticket's `priority` and `body` and a team's `description`. Creating a ticket with `"priority": ""` means no priority; leaving it out still defaults to medium.
- The README states that `MODE=worker` and the read-replica slot do nothing yet, and no longer hard-codes a version.

### Fixed
- The v1 ticket reads report `is_blocked`, `blocked_by`, `is_waiting` and `waiting_on`. They were always false and empty.
- Updating a team through the API without a `description` no longer clears it.

### Removed
- The unused WebSocket hub.

## [0.16.0] - 2026-10-21

### Upgrade notes
- **`JWT_SECRET` must be at least 32 bytes and not an example value**, or Docket refuses to start. Generate one with `openssl rand -hex 32`. On a fresh database `SEED_PASSWORD` must also be set to something other than `changeme`.
- **Update your Deployment** from `deploy/k8s/deployment.yaml`: probes now use the health port `9413`, and the termination grace period covers the drain. Previous probes hit `/login` on the app port.
- This release adds two migrations. From this release on, an older image started against a newer schema logs a warning and runs instead of crash-looping; rolling back to 0.15.0 or earlier still crash-loops.

### Security
- Logging out ends the whole session on every device that shares it, and a refresh token replayed after its grace window revokes the session and logs a warning. Existing sessions keep working.
- Docket refuses to start with a missing, short or placeholder `JWT_SECRET`, or with an example `SEED_PASSWORD` on a fresh database.
- Column lookups, ticket numbering and ticket history are scoped to the caller's organisation in the store as well as above it.

### Added
- Health endpoints on their own port (`9413`, set with `HEALTH_PORT`/`HEALTH_ADDR`): `/livez` reports that the process answers and never touches the database; `/readyz` checks that migrations finished, Postgres answers within a second, and the pod isn't shutting down. `?verbose` lists each check. The port is not part of the Service or the public route.
- `docket healthcheck` (`--live` for `/livez`) so the distroless image has a working Docker `HEALTHCHECK`; `deploy/docker-compose.yml` uses it.
- Closing a ticket asks for a reason: Done, Duplicate, Not relevant, Won't do, or Other with a note. The closed banner and the activity show it.
- The New Ticket dialog has a Track picker.
- Workspace settings has a Columns tab to add, rename, reorder and delete columns on any board.
- Kanban and Blank workspace dashboards show the board's flow: open, in progress, done in the last 7 days, open points, and open tickets per column.
- The Daily Scrum works on Kanban and Blank boards, over the work in progress.

### Changed
- On SIGTERM a replica fails `/readyz` immediately and keeps serving for `SHUTDOWN_DELAY` (default `5s`) before it stops, so rolling updates drain instead of dropping requests.
- The Kubernetes manifests probe the health port: a startup probe covers migrations, liveness uses `/livez`, readiness uses `/readyz`, and the termination grace period covers the drain.
- `.env.example`, the README, `deploy/docker-compose.yml` and `deploy/k8s/secret.yaml` document the secret requirements; compose no longer defaults `SEED_PASSWORD` to `changeme`.
- The refinement view is called "Backlog Refinement" everywhere.
- The board's always-visible "+ Add column" card is replaced by a small link to the Columns settings.
- On a Scrum board with no active sprint, the Daily Scrum explains why and links to Sprint Planning.
- On the Backlog page the active sprint is a drop target: dragging a ticket in warns that it is unplanned work. Tickets can be dragged between sprints.
- The roadmap and retrospective list load faster on boards with many sprints.

### Fixed
- Timestamps and their tooltips show in your browser's local time instead of the server's (UTC).
- Ticket and user picker searches match `%` and `_` literally.
- Whoever creates a workspace becomes a member of it. Existing workspaces get their creator added.
- Tickets in a sprint can be dragged back to the backlog after a page reload.
- Blocked and depends-on markers show in the backlog, sprint planning and sprint sections, and stay on a ticket after it is dragged.
- Repeatedly dragging tickets to the top of the backlog no longer drops them in a random spot.
- A ticket closed with a reason no longer reopens when it is dragged between columns; only leaving Done reopens it.
- A column that looks empty on the Sprint Board can be deleted. If visible tickets block it, the message says where they are.

## [0.15.0] - 2026-10-16

### Security
- Every route now declares the access it requires (read, write, org admin or metrics), and one place checks it against the caller's role and token scope. Anything not explicitly allowed is denied, and a test fails if a route is added without a declared tier. Behaviour is unchanged: members can do what they could before, and adding members or changing roles still needs an org admin. Denied requests are logged with the route and tier.

### Fixed
- Unassigned tickets take part in the Daily Scrum. They were counted in the participant list but could never be opened, and Next person skipped them; "Unassigned" is now the last turn of the round.
- Anyone assigned a ticket in the sprint comes up in the Daily Scrum round, not only the workspace's members.

## [0.14.3] - 2026-09-28

### Fixed
- The New Ticket dialog clears itself after creating a ticket, and closes when "Add another" is off. The code that ran after a successful create failed on its first line, so the fields kept the ticket just filed and the dialog stayed open; the redirect removed in 0.14.2 had been hiding it.
- Priority in the New Ticket dialog is a native dropdown with the same colour marker as the ticket drawer.

### Changed
- The "Ticket created" notice appears at the bottom centre with a check mark, as in the design.

### Development
- `make dev-docker` builds again (the dev image pins `air`, whose latest release needs a newer Go), and the stack live-reloads: the Tailwind and esbuild watchers no longer exit at startup, esbuild runs from the container's own `node_modules` and also rebuilds `editor.js`, and Air restarts on template changes.

## [0.14.2] - 2026-09-28

### Fixed
- Ticking an acceptance-criteria item ticks that item. Toggling counted every `[ ]` and `[x]` anywhere in the text, so brackets inside an item (`a[x]`) or on a line that isn't an item shifted the count and flipped the wrong box. Toggling, deleting and the list itself now share one definition of an item.
- A column can no longer be renamed to "Done", and a board can't get a second Done column. Either created a Done column that could never be renamed or deleted again. Column names are trimmed, and a blank name is rejected.
- Creating a link that already exists succeeds and returns the existing link instead of failing with a 500, and records no duplicate history.
- The New Ticket dialog's priority menu shows all five options; it was clipped after the first two. Story points are picked from the same 1/2/3/5/8/13 scale as on the ticket page instead of typed in.
- Changing a ticket's status from its page or the drawer refreshes the view, so a ticket moved to Done switches to its closed, read-only state straight away.
- Workspace key badges fit keys longer than four characters, on the workspace list and in the New Ticket workspace picker.

### Changed
- Creating a ticket keeps you where you are: the dialog closes (or clears, with "Add another") and a notice links to the new ticket, instead of navigating to it.
- The workspace dashboard is headed with the workspace's name instead of "Dashboard".

## [0.14.1] - 2026-09-28

### Fixed
- You can sign out again. The link was lost in the layout rewrite, leaving `POST /logout` with nothing that called it; there is now a Sign out button in the sidebar and on your own profile page.
- Deleting a workspace works. The button submitted a form POST with a hidden `_method=DELETE`, which nothing on the server reads, so it answered 405. Behind that, the delete itself failed on any workspace with tickets: the tickets' reference to their workspace had no cascade, and the database checked it before the board's cascade removed them. A migration makes that reference cascade.
- A sprint's name and goal can be set from Sprint Planning. The fields started empty and pressing Enter posted to a route that only accepts PUT, so neither could ever be saved. They now show the current values and save when you leave the field or press Enter. Starting a sprint applies the dates in the duration picker even if the sprint already had some.
- "Add another" in the New Ticket dialog keeps the dialog open and clears it for the next ticket, instead of navigating to the ticket just created.
- Moving a ticket to another column by drag and drop is recorded in the ticket's history, as it already was when changing the status from the ticket itself.

### Changed
- Responses are compressed. Static JavaScript and CSS, pages and HTMX partials were all sent uncompressed — about 280 KB of assets on a cold load that now arrive as about 80 KB.

## [0.14.0] - 2026-09-11

### Security
- The rich editor no longer runs markup from the text it opens. Switching to the visual pane turned the ticket or comment markdown into HTML with `marked` and set it as the pane's content, and `marked` passes raw HTML and `javascript:` links straight through — so a description containing an `<img onerror>` tag ran in the browser of whoever opened it for editing. Raw HTML is now shown as text and only `http`, `https`, `mailto` and relative links survive, matching what the server renderer already allowed.
- Auto-seeding runs only on an empty database. It used to be attempted on every start and relied on the insert colliding with the existing admin, so after changing `SEED_USERNAME` — or dropping the seed variables once the instance was set up — the next restart quietly created an `admin` account with the password `changeme`.
- Only an org admin or the token's creator can revoke an API token. Any member could revoke any token, including an admin's or an integration's.
- The last admin of an organisation can no longer be demoted, which left nobody able to manage members or roles.
- Writes that take an ID from the request now check that it belongs to the caller's organisation, and where it matters to the same board: creating columns, sprints, tracks and Definition of Done items; assigning a ticket to a sprint (which also rejects completed sprints); moving a ticket to a column; adding team members; setting sprint capacity; checking DoD items; adding and stacking retro cards; and deleting links. None exposed another organisation's data to the caller, but several stored rows pointing into it — a team member from another organisation was listed with their name and email.
- Assignees must be users of the caller's organisation, in the UI and in `PUT /api/v1/tickets/{ref}`.
- Request bodies are capped at 2 MiB.
- Error responses no longer echo internal error text, which for some sprint and retro actions included database messages. Viewing an unknown ticket by reference returned `no rows in result set` as a 400; it is now a 404.

### Fixed
- Deleting a ticket from its page now returns to the workspace board instead of a route that does not exist.
- Re-assigning the owner of a retro action item reuses its backlog ticket instead of filing a new one each time.
- Stacking a retro card that already had cards stacked on it no longer makes those cards disappear.
- Opening a retro via a URL whose workspace did not match no longer creates an empty retro.
- Editing a ticket's title or description in the UI no longer clears its legacy `assignee_id`.
- A column that still holds tickets, or a board's last column, can no longer be deleted — both used to fail with a bare 500. Renaming or deleting the Done column now answers 400 rather than 500.
- Only a sprint still in planning can be deleted, and completed sprints can no longer be edited. Starting a sprint while another is active now explains why instead of failing with a 500.
- A ticket can no longer be linked to itself.
- Creating or renaming a workspace to a name or key already in use answers `409` instead of `500`, as does a duplicate track name. A workspace name that is blank, or a key with no letters or digits, is rejected.
- Story points must be a non-negative number everywhere, including the New Ticket dialog.
- Closing a sprint is now all-or-nothing. It ran as several separate steps, so a failure partway through could leave the unfinished tickets back in the backlog while the sprint stayed active — and a retry then recorded the sprint's numbers without them. Two people closing the same sprint at once no longer both run the close either.
- `@username` mentions link to a page that exists. They always pointed at `/users/{username}`, which nothing served; it now shows that member's profile card.
- The revoke button on API tokens only appears for tokens you are allowed to revoke — your own, or any if you are an admin.

## [0.13.5] - 2026-09-10

### Fixed
- Ticket priority is now validated. The column had no constraint, so the public API accepted any string (`"urgent"`), which sorted last and rendered without a badge. Unknown values are rejected with `400` in both the UI and `/api/v1`.
- Creating a ticket now checks that the column belongs to the board and the board to the caller's organisation. `POST /api/v1/teams/{key}/tickets` takes both IDs from the request body, so a token from one organisation could file a ticket into another's column. It also rejects a `board_id` that is not the named team's own board, which would have numbered the ticket under one team and shown it on another's board.
- A sprint can no longer be given an end date before its start date.
- `GET /api/v1/teams` and `GET /api/v1/teams/{key}/tickets` return `[]` rather than `null` when there is nothing to list.

## [0.13.4] - 2026-08-10

### Fixed
- Sessions no longer end at random. Refreshing an expired access token rotated the refresh token by revoking it outright, so when several requests hit the 15-minute expiry together — which the board's 30s poll makes routine — the first rotated the token and the rest presented one revoked microseconds earlier and were sent to the login page, while a sibling request was busy establishing a valid session. A rotated token now stays usable for a short grace period. Logout still revokes immediately.
- `COOKIE_SECURE` was only honoured when signing in. The session cookies written when the middleware silently refreshed a token were always marked `Secure`, so on an HTTP instance the browser discarded them 15 minutes after login and the session was gone for good. Session cookies now have a single writer that carries the configured flag.
- An expired session no longer paints the login form inside part of the page. Unauthenticated requests were answered with a redirect, which htmx follows before swapping the response into whatever element triggered it — a background poll would render the login page inside the board while the sidebar and navigation stayed put. Requests from htmx now get a client-side redirect and the whole page navigates.
- Adding a member whose username already existed silently overwrote that account's name and email, and reset its password. It is now rejected, as is a duplicate email — which previously failed with a raw 500. Neither ever created a user, but the first quietly took over an existing one.
- Creating an API token or adding a member replaced the whole page with an unstyled fragment. Both forms submitted natively to handlers that return a page fragment; they are now htmx requests that update their panel in place.
- Revoking an API token and changing a member's role did nothing at all. Both submitted `POST` to routes registered as `DELETE` and `PUT`, and there is no method-override middleware, so every attempt was a `405` with no feedback.
- The Docket logo in the top left is now a link to the organisation's start page.

## [0.13.3] - 2026-07-30

### Added
- Sprint metrics gained `docket_sprint_remaining_points` plus `docket_sprint_start_timestamp_seconds` and `docket_sprint_end_timestamp_seconds` (the latter two only when the sprint has dates). Together these make a real burndown possible: remaining points against an ideal line derived from the sprint's actual timebox rather than from whatever range the dashboard happens to show.

## [0.13.2] - 2026-07-30

### Added
- `COOKIE_SECURE` (default `true`) controls whether session cookies are marked `Secure`. Browsers refuse to store `Secure` cookies over plain HTTP, so an HTTP-only instance previously looked like it rejected valid credentials: the server issued a session and the browser discarded it. Set it to `false` only where the instance is reachable over HTTP on a trusted network.

## [0.13.1] - 2026-07-30

### Fixed
- The published Docker image could not start. The distroless nonroot base sets its working directory to `/home/nonroot`, which does not exist in the static variant, and the image never overrode it — so resolving `migrations/`, `templates/` and `static/dist` failed immediately. This affected every release up to and including 0.13.0; the local development stack was unaffected, which is why it went unnoticed. Anyone self-hosting from the published image needs this release.

## [0.13.0] - 2026-07-29

### Added
- Ticket dependencies are their own relationship, separate from blocking. A block is a hard stop; a dependency means "this needs doing first" without anyone necessarily being stuck. The vocabulary is now Blocks / Blocked by, Depends on / Required by, Duplicates / Duplicated by, and Relates to.
- Dependencies are marked amber on board cards and in the Daily Scrum, rather than the red reserved for blocks — a waiting ticket is not free to pick up, but it is not an emergency either. The marker clears when the depended-on ticket closes, while the link itself is kept.
- The link picker offers both directions of every relationship; choosing the inverse phrasing stores the same single row with the tickets swapped.
- Tickets returned by the public API carry their tracks, so `GET /api/v1/teams/{key}/tickets` can answer which tickets belong to a track.

### Changed
- Relationship wording lives in one place in the model rather than being re-spelled in the store, the service and the template.

### Fixed
- The backlog's track column rendered a header with no values under it: backlog tickets never had their tags loaded.
- A relationship read from the far ticket only inverted for blocks, so a duplicate link read "Duplicates" from both ends — backwards on one of them.
- Adding a link that points at a closed ticket failed from a perfectly open one, because the immutability guard checked the wrong end of the link.
- Ticket history described relationships with wording that no longer appeared anywhere in the UI.
- Workspace cards on the organisation dashboard showed the name twice ("Network network"), since the slug is derived from the name.

### Security
- Patched `golang.org/x/text` (GO-2026-5970), reachable through the database connection pool, and a high-severity path traversal in postcss.

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

[Unreleased]: https://github.com/allmend/docket/compare/v0.18.0...HEAD
[0.18.0]: https://github.com/allmend/docket/compare/v0.17.0...v0.18.0
[0.17.0]: https://github.com/allmend/docket/compare/v0.16.0...v0.17.0
[0.16.0]: https://github.com/allmend/docket/compare/v0.15.0...v0.16.0
[0.15.0]: https://github.com/allmend/docket/compare/v0.14.3...v0.15.0
[0.14.3]: https://github.com/allmend/docket/compare/v0.14.2...v0.14.3
[0.14.2]: https://github.com/allmend/docket/compare/v0.14.1...v0.14.2
[0.14.1]: https://github.com/allmend/docket/compare/v0.14.0...v0.14.1
[0.14.0]: https://github.com/allmend/docket/compare/v0.13.5...v0.14.0
[0.13.5]: https://github.com/allmend/docket/compare/v0.13.4...v0.13.5
[0.13.4]: https://github.com/allmend/docket/compare/v0.13.3...v0.13.4
[0.13.3]: https://github.com/allmend/docket/compare/v0.13.2...v0.13.3
[0.13.2]: https://github.com/allmend/docket/compare/v0.13.1...v0.13.2
[0.13.1]: https://github.com/allmend/docket/compare/v0.13.0...v0.13.1
[0.13.0]: https://github.com/allmend/docket/compare/v0.12.0...v0.13.0
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
