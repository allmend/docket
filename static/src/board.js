/**
 * board.js — SortableJS wiring for the Docket kanban/scrum board and backlog page.
 *
 * board(boardID, sprintID):
 *   Used on the board view. Wires sprint columns as full drag targets and the
 *   virtual backlog column as a source-only list.
 *   - Sprint column drop: POST /tickets/:id/move
 *   - Backlog → sprint column drop: POST /tickets/:id/sprint-place (assigns sprint + moves)
 *
 * backlogList():
 *   Used on the backlog page. Wires the flat ticket list for position reordering.
 *   - Drop: POST /tickets/:id/move (keeps same column_id, updates position only)
 */

import Sortable from "sortablejs";

// SortableJS sets this.el = null on destroy(), but stale dragover listeners can
// still fire afterward. Patch the prototype so _onDragOver is a no-op when the
// instance has been destroyed, preventing "lastElementChild of null" crashes.
{
  const orig = Sortable.prototype._onDragOver;
  if (orig) {
    Sortable.prototype._onDragOver = function (evt) {
      if (this.el) orig.call(this, evt);
    };
  }
}

function board(boardID, sprintID) {
  return {
    boardID,
    sprintID,
    sortables: [],

    filterPriorities: [],
    filterAssignees: [],
    filterMaxAgeDays: 0,

    init() {
      this._loadFromURL();
      this.initSortables();
      if (this.activeFilterCount() > 0) this.applyFilters();

      // Only reinitialize when the board columns container is swapped.
      // Firing on every htmx:afterSwap (modal loads, comment posts, etc.) destroys
      // and recreates Sortables unnecessarily, and if that happens mid-drag,
      // SortableJS's _onDragOver accesses a null parentNode and crashes.
      document.addEventListener("htmx:afterSwap", (e) => {
        const t = e.detail.target;
        if (!(t && (t.id === "board-columns" || t.querySelector?.(".ticket-list, .backlog-ticket-list")))) return;
        if (Sortable.dragged) {
          // Board swapped while a drag is active — defer until drag ends.
          document.addEventListener("dragend", () => { this.initSortables(); this.applyFilters(); }, { once: true });
        } else {
          this.initSortables();
          this.applyFilters();
        }
      });
    },

    // Return sorted unique assignee names from all ticket cards on the board.
    boardAssignees() {
      const seen = new Set();
      document.querySelectorAll("[data-assignees]").forEach((el) => {
        (el.dataset.assignees || "").split("|").filter(Boolean).forEach((n) => seen.add(n));
      });
      return [...seen].sort();
    },

    activeFilterCount() {
      return this.filterPriorities.length + this.filterAssignees.length + (this.filterMaxAgeDays > 0 ? 1 : 0);
    },

    togglePriority(p) {
      const idx = this.filterPriorities.indexOf(p);
      if (idx === -1) this.filterPriorities.push(p);
      else this.filterPriorities.splice(idx, 1);
      this.applyFilters();
    },

    toggleAssignee(name) {
      const idx = this.filterAssignees.indexOf(name);
      if (idx === -1) this.filterAssignees.push(name);
      else this.filterAssignees.splice(idx, 1);
      this.applyFilters();
    },

    clearFilters() {
      this.filterPriorities = [];
      this.filterAssignees = [];
      this.filterMaxAgeDays = 0;
      this.applyFilters();
    },

    // Persist filter state in the URL so filters survive reload and are shareable.
    _syncURL() {
      const url = new URL(window.location.href);
      if (this.filterPriorities.length) url.searchParams.set("priority", this.filterPriorities.join(","));
      else url.searchParams.delete("priority");
      if (this.filterAssignees.length) url.searchParams.set("assignees", this.filterAssignees.join("|"));
      else url.searchParams.delete("assignees");
      if (this.filterMaxAgeDays > 0) url.searchParams.set("age", this.filterMaxAgeDays);
      else url.searchParams.delete("age");
      history.replaceState(null, "", url.toString());
    },

    _loadFromURL() {
      const p = new URLSearchParams(window.location.search);
      const priority = p.get("priority");
      if (priority) this.filterPriorities = priority.split(",").filter(Boolean);
      const assignees = p.get("assignees");
      if (assignees) this.filterAssignees = assignees.split("|").filter(Boolean);
      const age = p.get("age");
      if (age) this.filterMaxAgeDays = parseInt(age) || 0;
    },

    applyFilters() {
      const now = Date.now() / 1000;
      const hasPriority = this.filterPriorities.length > 0;
      const hasAssignee = this.filterAssignees.length > 0;
      const hasAge = this.filterMaxAgeDays > 0;

      document.querySelectorAll(".ticket-list, .backlog-ticket-list").forEach((list) => {
        let visibleCount = 0;
        list.querySelectorAll("[data-ticket-id]").forEach((card) => {
          let show = true;
          if (hasPriority && !this.filterPriorities.includes(card.dataset.priority || "")) show = false;
          if (show && hasAssignee) {
            const cardNames = (card.dataset.assignees || "").split("|").filter(Boolean);
            if (!this.filterAssignees.some((n) => cardNames.includes(n))) show = false;
          }
          if (show && hasAge) {
            const ageDays = (now - parseInt(card.dataset.createdUnix || 0)) / 86400;
            // Age filter = max age: hide tickets OLDER than N days.
            if (ageDays > this.filterMaxAgeDays) show = false;
          }
          card.classList.toggle("hidden", !show);
          if (show) visibleCount++;
        });
        const badge = list.parentElement?.querySelector("[data-column-count]");
        if (badge) badge.textContent = visibleCount;
      });

      this._syncURL();
    },

    initSortables() {
      // Calling Sortable.create while a drag is in progress corrupts SortableJS's
      // internal state: the global _onDragOver handler fires during construction
      // and tries to access a parentNode that the ongoing drag has set to null.
      if (Sortable.dragged) return;

      this.sortables.forEach((s) => { try { s.destroy(); } catch (_) {} });
      this.sortables = [];

      // Backlog column — source-only: can drag out, cannot drop in.
      const backlogEl = document.querySelector(".backlog-ticket-list");
      if (backlogEl) {
        try {
          this.sortables.push(Sortable.create(backlogEl, {
            group: { name: "tickets", pull: true, put: true },
            animation: 150,
            ghostClass: "opacity-30",
            dragClass: "shadow-2xl",
            onEnd: (evt) => this.onDrop(evt),
          }));
        } catch (_) {}
      }

      // Sprint / kanban columns — normal drag targets.
      document.querySelectorAll(".ticket-list").forEach((el) => {
        try {
          this.sortables.push(Sortable.create(el, {
            group: "tickets",
            animation: 150,
            ghostClass: "opacity-30",
            dragClass: "shadow-2xl",
            onEnd: (evt) => this.onDrop(evt),
          }));
        } catch (_) {}
      });
    },

    onDrop(evt) {
      const ticketEl = evt.item;
      const ticketID = ticketEl.dataset.ticketId;
      const toEl = evt.to;
      const fromEl = evt.from;
      const toColumnID = toEl.dataset.columnId;
      const fromColumnID = fromEl.dataset.columnId;

      const siblings = Array.from(toEl.querySelectorAll("[data-ticket-id]"));
      const idx = siblings.indexOf(ticketEl);
      const prevPos = idx > 0
        ? parseFloat(siblings[idx - 1].dataset.position || "0")
        : 0;
      const nextPos = idx < siblings.length - 1
        ? parseFloat(siblings[idx + 1].dataset.position || "0")
        : 0;

      if (toColumnID === "backlog" && fromColumnID === "backlog") {
        // Reorder within the backlog — persist position only.
        const ticketColumnID = ticketEl.dataset.ticketColumnId;
        const params = new URLSearchParams({
          column_id: ticketColumnID,
          prev_pos: String(prevPos),
          next_pos: String(nextPos),
        });
        fetch(`/tickets/${ticketID}/move`, { method: "POST", body: params })
          .then((res) => {
            if (!res.ok) console.error("backlog reorder failed", res.status);
          });
      } else if (toColumnID === "backlog") {
        // Returning from a sprint column to the backlog — un-assign from sprint.
        const params = new URLSearchParams({ sprint_id: "" });
        fetch(`/tickets/${ticketID}/sprint`, { method: "POST", body: params })
          .then((res) => {
            if (!res.ok) console.error("sprint-unassign failed", res.status);
            else { this.updateColumnCounts(fromEl, toEl); this.updateDoneClass(ticketEl, toEl); }
          });
      } else if (this.sprintID) {
        // Moving into a sprint column — always use sprint-place so sprint_id is set
        // regardless of whether the ticket came from the backlog or another sprint column.
        const params = new URLSearchParams({
          sprint_id: this.sprintID,
          column_id: toColumnID,
          prev_pos: String(prevPos),
          next_pos: String(nextPos),
        });
        fetch(`/tickets/${ticketID}/sprint-place`, { method: "POST", body: params })
          .then((res) => {
            if (!res.ok) console.error("sprint-place failed", res.status);
            else { this.updateColumnCounts(fromEl, toEl); this.updateDoneClass(ticketEl, toEl); }
          });
      } else {
        // Kanban / blank board — plain column move.
        const params = new URLSearchParams({
          column_id: toColumnID,
          prev_pos: String(prevPos),
          next_pos: String(nextPos),
        });
        fetch(`/tickets/${ticketID}/move`, { method: "POST", body: params })
          .then((res) => {
            if (!res.ok) console.error("move failed", res.status);
            else { this.updateColumnCounts(fromEl, toEl); this.updateDoneClass(ticketEl, toEl); }
          });
      }
    },

    updateColumnCounts(fromEl, toEl) {
      const update = (listEl) => {
        const count = listEl.querySelectorAll("[data-ticket-id]").length;
        const badge = listEl.parentElement?.querySelector("[data-column-count]");
        if (badge) badge.textContent = count;
      };
      update(fromEl);
      if (toEl !== fromEl) update(toEl);
    },

    updateDoneClass(ticketEl, toEl) {
      const isDone = toEl.dataset.isDone === "true";
      ticketEl.classList.toggle("opacity-50", isDone);

      // Sync the "closed" chip. The server sets/clears closed_at on column moves,
      // but the card DOM isn't re-rendered — we mirror the change client-side.
      const metaRow = ticketEl.querySelector(".flex.items-center.gap-2");
      if (!metaRow) return;
      const existing = metaRow.querySelector("[data-closed-chip]");
      if (isDone && !existing) {
        const link = metaRow.querySelector("a");
        if (link) {
          const chip = document.createElement("span");
          chip.setAttribute("data-closed-chip", "");
          chip.className = "text-xs border border-amber-800/50 text-amber-600/70 rounded-md px-1.5 py-0.5";
          chip.textContent = "closed";
          link.insertAdjacentElement("afterend", chip);
        }
      } else if (!isDone && existing) {
        existing.remove();
      }
    },
  };
}

// Show an informational banner above #backlog-ticket-list. Dismissed with X.
function showSprintWarning(message) {
  document.getElementById("sprint-warning")?.remove();
  const bar = document.createElement("div");
  bar.id = "sprint-warning";
  bar.className = "flex items-center gap-3 px-4 py-3 bg-base-200 border border-amber-700 rounded-lg text-sm text-amber-400 mb-3";
  bar.innerHTML = `
    <svg class="h-4 w-4 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
      <path stroke-linecap="round" stroke-linejoin="round" d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126zM12 15.75h.007v.008H12v-.008z"/>
    </svg>
    <span class="flex-1">${message}</span>
    <button class="text-amber-400/60 hover:text-amber-400 transition-colors" onclick="document.getElementById('sprint-warning').remove()">
      <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12"/></svg>
    </button>
  `;
  const list = document.getElementById("backlog-ticket-list");
  if (list) list.parentNode.insertBefore(bar, list);
}

// backlogList — wires the flat backlog ticket list for drag-to-reorder.
// Each ticket row must have data-ticket-id, data-position, data-ticket-column-id.
function backlogList() {
  return {
    sortable: null,
    sprintSortables: [],
    _swapHandler: null,

    init() {
      this.initSortables();
      this._swapHandler = (e) => {
        if (e.detail.target && e.detail.target.id === "backlog-ticket-list") {
          this.initSortables();
        }
      };
      document.addEventListener("htmx:afterSwap", this._swapHandler);
    },

    destroy() {
      if (this._swapHandler) {
        document.removeEventListener("htmx:afterSwap", this._swapHandler);
        this._swapHandler = null;
      }
      if (this.sortable) { this.sortable.destroy(); this.sortable = null; }
      this.sprintSortables.forEach((s) => { try { s.destroy(); } catch (_) {} });
      this.sprintSortables = [];
    },

    initSortables() {
      if (this.sortable) { this.sortable.destroy(); this.sortable = null; }
      this.sprintSortables.forEach((s) => { try { s.destroy(); } catch (_) {} });
      this.sprintSortables = [];

      document.querySelectorAll(".sprint-list").forEach((sprintEl) => {
        this.sprintSortables.push(Sortable.create(sprintEl, {
          group: "backlog-sprint",
          animation: 150,
          ghostClass: "opacity-30",
          dragClass: "shadow-2xl",
          handle: ".drag-handle",
          onEnd: (evt) => this.onDrop(evt),
        }));
      });

      const backlogEl = document.querySelector(".backlog-list");
      if (!backlogEl) return;
      this.sortable = Sortable.create(backlogEl, {
        group: "backlog-sprint",
        animation: 150,
        ghostClass: "opacity-30",
        dragClass: "shadow-2xl",
        handle: ".drag-handle",
        onEnd: (evt) => this.onDrop(evt),
      });
    },

    onDrop(evt) {
      const ticketEl = evt.item;
      const ticketID = ticketEl.dataset.ticketId;
      const fromSprint = evt.from.classList.contains("sprint-list");
      const toSprint = evt.to.classList.contains("sprint-list");

      if (!fromSprint && toSprint) {
        // Backlog → sprint: place immediately, warn only if the sprint is active.
        const sprintID = evt.to.dataset.sprintId;
        const sprintStatus = evt.to.dataset.sprintStatus;
        const siblings = Array.from(evt.to.querySelectorAll("[data-ticket-id]"));
        const idx = siblings.indexOf(ticketEl);
        const prevPos = idx > 0 ? parseFloat(siblings[idx - 1].dataset.position || "0") : 0;
        const nextPos = idx < siblings.length - 1 ? parseFloat(siblings[idx + 1].dataset.position || "0") : 0;
        const nextTick = idx < siblings.length - 1 ? siblings[idx + 1] : null;
        const prevTick = idx > 0 ? siblings[idx - 1] : null;
        const columnID = (nextTick || prevTick)?.dataset.ticketColumnId || evt.to.dataset.firstColumnId || "";
        const isActive = sprintStatus === "active";
        const params = new URLSearchParams({ sprint_id: sprintID, column_id: columnID, prev_pos: String(prevPos), next_pos: String(nextPos) });
        if (isActive) params.set("unplanned", "1");
        fetch(`/tickets/${ticketID}/sprint-place`, { method: "POST", body: params })
          .then((res) => {
            if (!res.ok) console.error("sprint-place failed", res.status);
            else {
              if (isActive) showSprintWarning("Sprint commitment is broken — the active sprint was modified.");
              document.body.dispatchEvent(new CustomEvent("boardUpdated"));
            }
          });
      } else if (fromSprint && !toSprint) {
        // Sprint → backlog: unassign immediately, warn only if sprint was active.
        const sprintStatus = evt.from.dataset.sprintStatus;
        const isActive = sprintStatus === "active";
        const ticketColumnID = ticketEl.dataset.ticketColumnId;
        const siblings = Array.from(evt.to.querySelectorAll("[data-ticket-id]"));
        const idx = siblings.indexOf(ticketEl);
        const prevPos = idx > 0 ? parseFloat(siblings[idx - 1].dataset.position || "0") : 0;
        const nextPos = idx < siblings.length - 1 ? parseFloat(siblings[idx + 1].dataset.position || "0") : 0;
        fetch(`/tickets/${ticketID}/sprint`, {
          method: "POST",
          body: new URLSearchParams({ sprint_id: "" }),
        }).then((res) => {
          if (!res.ok) { console.error("sprint-unassign failed", res.status); return null; }
          return fetch(`/tickets/${ticketID}/move`, {
            method: "POST",
            body: new URLSearchParams({ column_id: ticketColumnID, prev_pos: String(prevPos), next_pos: String(nextPos) }),
          });
        }).then((res) => {
          if (res && !res.ok) console.error("position failed", res.status);
          else if (res) {
            if (isActive) showSprintWarning("Sprint commitment is broken — the active sprint was modified.");
            document.body.dispatchEvent(new CustomEvent("boardUpdated"));
          }
        });
      } else {
        // Reorder within same list (backlog or sprint)
        const ticketColumnID = ticketEl.dataset.ticketColumnId;
        const siblings = Array.from(evt.to.querySelectorAll("[data-ticket-id]"));
        const idx = siblings.indexOf(ticketEl);
        const prevPos = idx > 0 ? parseFloat(siblings[idx - 1].dataset.position || "0") : 0;
        const nextPos = idx < siblings.length - 1 ? parseFloat(siblings[idx + 1].dataset.position || "0") : 0;
        fetch(`/tickets/${ticketID}/move`, {
          method: "POST",
          body: new URLSearchParams({ column_id: ticketColumnID, prev_pos: String(prevPos), next_pos: String(nextPos) }),
        }).then((res) => { if (!res.ok) console.error("reorder failed", res.status); });
      }
    },
  };
}

window.board = board;
window.backlogList = backlogList;
