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

function board(boardID, sprintID) {
  return {
    boardID,
    sprintID,
    sortables: [],

    init() {
      this.initSortables();
      document.addEventListener("htmx:afterSwap", () => this.initSortables());
    },

    initSortables() {
      this.sortables.forEach((s) => s.destroy());
      this.sortables = [];

      // Backlog column — source-only: can drag out, cannot drop in.
      const backlogEl = document.querySelector(".backlog-ticket-list");
      if (backlogEl) {
        const s = Sortable.create(backlogEl, {
          group: { name: "tickets", pull: true, put: true },
          animation: 150,
          ghostClass: "opacity-30",
          dragClass: "shadow-2xl",
          onEnd: (evt) => this.onDrop(evt),
        });
        this.sortables.push(s);
      }

      // Sprint / kanban columns — normal drag targets.
      document.querySelectorAll(".ticket-list").forEach((el) => {
        const s = Sortable.create(el, {
          group: "tickets",
          animation: 150,
          ghostClass: "opacity-30",
          dragClass: "shadow-2xl",
          onEnd: (evt) => this.onDrop(evt),
        });
        this.sortables.push(s);
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
    },
  };
}

// backlogList — wires the flat backlog ticket list for drag-to-reorder.
// Each ticket row must have data-ticket-id, data-position, data-ticket-column-id.
function backlogList() {
  return {
    sortable: null,

    init() {
      this.initSortable();
      document.addEventListener("htmx:afterSwap", (e) => {
        if (e.detail.target && e.detail.target.id === "backlog-ticket-list") {
          this.initSortable();
        }
      });
    },

    initSortable() {
      if (this.sortable) { this.sortable.destroy(); this.sortable = null; }
      const el = document.querySelector(".backlog-list");
      if (!el) return;
      this.sortable = Sortable.create(el, {
        animation: 150,
        ghostClass: "opacity-30",
        dragClass: "shadow-2xl",
        handle: ".drag-handle",
        onEnd: (evt) => {
          const ticketEl = evt.item;
          const ticketID = ticketEl.dataset.ticketId;
          const ticketColumnID = ticketEl.dataset.ticketColumnId;
          const siblings = Array.from(el.querySelectorAll("[data-ticket-id]"));
          const idx = siblings.indexOf(ticketEl);
          const prevPos = idx > 0 ? parseFloat(siblings[idx - 1].dataset.position || "0") : 0;
          const nextPos = idx < siblings.length - 1 ? parseFloat(siblings[idx + 1].dataset.position || "0") : 0;
          fetch(`/tickets/${ticketID}/move`, {
            method: "POST",
            body: new URLSearchParams({ column_id: ticketColumnID, prev_pos: String(prevPos), next_pos: String(nextPos) }),
          }).then((res) => { if (!res.ok) console.error("reorder failed", res.status); });
        },
      });
    },
  };
}

window.board = board;
window.backlogList = backlogList;
