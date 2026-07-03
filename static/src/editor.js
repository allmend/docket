import { marked } from "marked";
import TurndownService from "turndown";

// Accent color for a code fence's language dot (mirrors internal/markdown).
function fenceDot(lang) {
  switch ((lang || "").toLowerCase()) {
    case "shell": case "sh": case "bash": case "zsh": case "console":
      return "#7ee787";
    case "json":
      return "#79c0ff";
    default:
      return "#10b981";
  }
}

marked.use({
  breaks: true,
  gfm: true,
  renderer: {
    // Codex-style code block shell — same markup the server renderer emits,
    // so the visual pane and the rendered ticket look identical. The header
    // is contenteditable=false: an atom the cursor skips over.
    code({ text, lang }) {
      const l = (lang || "").trim().split(/\s+/)[0];
      const head = l
        ? '<span class="pre-head" contenteditable="false"><span class="fdot" style="background:' +
          fenceDot(l) + '"></span>' + escHTML(l) + "</span>"
        : "";
      const cls = l ? ' class="language-' + escHTML(l) + '"' : "";
      return '<pre class="codeblock">' + head + "<code" + cls + ">" + escHTML(text) + "\n</code></pre>";
    },
  },
});

const td = new TurndownService({
  headingStyle: "atx",
  bulletListMarker: "-",
  codeBlockStyle: "fenced",
  emDelimiter: "*",
});

// Turndown's default pads list markers to four columns ("-   item").
// Emit the tight "- item" / "1. item" style instead.
td.addRule("listItem", {
  filter: "li",
  replacement: (content, node, options) => {
    content = content
      .replace(/^\n+/, "")
      .replace(/\n+$/, "\n")
      .replace(/\n/gm, "\n  ");
    let prefix = options.bulletListMarker + " ";
    const parent = node.parentNode;
    if (parent.nodeName === "OL") {
      const start = parent.getAttribute("start");
      const index = Array.prototype.indexOf.call(parent.children, node);
      prefix = (start ? Number(start) + index : index + 1) + ". ";
    }
    return prefix + content + (node.nextSibling && !/\n$/.test(content) ? "\n" : "");
  },
});

td.addRule("strikethrough", {
  filter: (node) =>
    node.nodeName === "DEL" || node.nodeName === "S" || node.nodeName === "STRIKE",
  replacement: (content) => `~~${content}~~`,
});

// Fenced code blocks may contain <br> elements (inserted by contenteditable
// when the user presses Enter inside a <pre>). Walk child nodes explicitly so
// both text-node newlines (from marked.parse) and <br> elements are preserved.
// The code element is found by query, not firstChild — the Codex-style
// .pre-head header sits before it and must not end up in the fence.
td.addRule("fenced-code-block", {
  filter: (node) => node.nodeName === "PRE" && !!node.querySelector("code"),
  replacement: (_content, node) => {
    const code = node.querySelector("code");
    // marked sets class="language-javascript" — preserve the lang identifier.
    const lang = (code.className || "").replace(/\blanguage-/, "").trim();
    let text = "";
    code.childNodes.forEach((child) => {
      if (child.nodeType === 3) text += child.textContent;
      else if (child.nodeName === "BR") text += "\n";
      else text += child.textContent;
    });
    return "\n\n```" + lang + "\n" + text.replace(/\n$/, "") + "\n```\n\n";
  },
});

function escHTML(s) {
  return s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/"/g, "&quot;");
}

function richEditor(fieldName) {
  const mention = typeof mentionInput === "function" ? mentionInput() : {};

  return {
    ...mention,

    fieldName,
    mode: "code",
    src: "",
    _lastRange: null,
    _cleanups: [],
    _STORAGE_KEY: "docket-editor-mode",

    isBold: false,
    isItalic: false,
    isStrike: false,
    isCode: false,
    blockType: "p",

    init() {
      this.$nextTick(() => {
        if (this.$refs.codearea) this.src = this.$refs.codearea.value;

        if (localStorage.getItem(this._STORAGE_KEY) === "visual") {
          this.switchToVisual();
        }

        // Sync visual pane → textarea before HTMX reads the form.
        // Using capture phase so this runs before HTMX's submit handler.
        const form = this.$el?.closest("form");
        if (form) {
          const onSubmit = () => {
            if (this.mode === "visual" && this.$refs.visual && this.$refs.codearea) {
              this._stripTrailingParagraph();
              // The ZWS strip clears cursor-anchor artifacts saved by the old editor.
              this.$refs.codearea.value = td.turndown((this.$refs.visual.innerHTML || "").replace(/​/g, ""));
              this._ensureTrailingParagraph();
            }
          };
          // After a successful comment post the form is reset — clear both panes.
          const onReset = () => {
            this.src = "";
            if (this.$refs.codearea) this.$refs.codearea.value = "";
            if (this.$refs.visual) this.$refs.visual.innerHTML = "";
          };
          form.addEventListener("submit", onSubmit, { capture: true });
          form.addEventListener("reset", onReset);
          this._cleanups.push(
            () => form.removeEventListener("submit", onSubmit, { capture: true }),
            () => form.removeEventListener("reset", onReset)
          );
        }
      });

      // Track last selection within visual pane for toolbar state + mention insertion.
      const onSel = () => {
        if (this.mode !== "visual" || !this.$refs.visual) return;
        const sel = window.getSelection();
        if (!sel.rangeCount) return;
        const container = sel.getRangeAt(0).commonAncestorContainer;
        if (!this.$refs.visual.contains(container) && container !== this.$refs.visual) return;
        this._lastRange = sel.getRangeAt(0).cloneRange();
        this._updateToolbar(container);
      };
      document.addEventListener("selectionchange", onSel);
      this._cleanups.push(() => document.removeEventListener("selectionchange", onSel));
    },

    destroy() {
      this._cleanups.forEach((fn) => fn());
    },

    // Mode switching

    switchToVisual() {
      if (this.mode === "visual") return;
      const h = this.$refs.codearea ? this.$refs.codearea.offsetHeight : 0;
      if (this.$refs.codearea) this.src = this.$refs.codearea.value;
      this.mode = "visual";
      localStorage.setItem(this._STORAGE_KEY, "visual");
      this.$nextTick(() => {
        if (!this.$refs.visual) return;
        // Explicit height so the pane doesn't jump when toggling modes;
        // both panes are resize-y so the user can still adjust.
        if (h > 0) this.$refs.visual.style.height = h + "px";
        // New blocks come out as <p>, not <div> — keeps turndown output clean.
        try {
          document.execCommand("defaultParagraphSeparator", false, "p");
          document.execCommand("styleWithCSS", false, false);
        } catch (_) {}
        this.$refs.visual.innerHTML = marked.parse(this.src || "") || "<p><br></p>";
        this._ensureTrailingParagraph();
        this.$refs.visual.focus();
      });
    },

    switchToCode() {
      if (this.mode === "code") return;
      const h = this.$refs.visual ? this.$refs.visual.offsetHeight : 0;
      if (this.$refs.visual) {
        this._stripTrailingParagraph();
        this.src = td.turndown((this.$refs.visual.innerHTML || "").replace(/​/g, ""));
      }
      this.mode = "code";
      localStorage.setItem(this._STORAGE_KEY, "code");
      // No selection tracking in code mode — clear the visual-mode button states.
      this.isBold = this.isItalic = this.isStrike = this.isCode = false;
      this.blockType = "p";
      this.$nextTick(() => {
        if (!this.$refs.codearea) return;
        if (h > 0) this.$refs.codearea.style.height = h + "px";
        this.$refs.codearea.value = this.src;
        this.$refs.codearea.focus();
      });
    },

    // Toolbar state

    _updateToolbar(container) {
      const v = this.$refs.visual;
      this.isBold = document.queryCommandState("bold");
      this.isItalic = document.queryCommandState("italic");
      this.isStrike = document.queryCommandState("strikeThrough");
      this.isCode = !!this._ancestor(container, v, (n) => n.tagName === "CODE");
      const block = this._ancestor(container, v, (n) =>
        ["P", "H1", "H2", "H3", "H4", "H5", "H6", "LI", "BLOCKQUOTE", "PRE"].includes(n.tagName)
      );
      this.blockType = block ? block.tagName.toLowerCase() : "p";
    },

    _refreshToolbar() {
      const sel = window.getSelection();
      if (sel && sel.rangeCount) this._updateToolbar(sel.getRangeAt(0).commonAncestorContainer);
    },

    // Formatting actions — visual mode uses the browser's native editing
    // engine (execCommand) so typing, Enter, and undo all behave natively.

    toggleInline(tag) {
      if (this.mode === "code") {
        const marker = { strong: "**", em: "*", s: "~~", code: "`" }[tag];
        if (marker) this._mdInline(marker);
        return;
      }
      this._restoreSel();
      const cmd = { strong: "bold", em: "italic", s: "strikeThrough" }[tag];
      if (cmd) document.execCommand(cmd);
      else if (tag === "code") this._toggleInlineCode();
      this.$refs.visual.focus();
      this._refreshToolbar();
    },

    _toggleInlineCode() {
      const visual = this.$refs.visual;
      const sel = window.getSelection();
      if (!sel || !sel.rangeCount) return;
      const existing = this._ancestor(
        sel.getRangeAt(0).commonAncestorContainer,
        visual,
        (n) => n.tagName === "CODE"
      );
      if (existing && existing.parentElement && existing.parentElement.tagName !== "PRE") {
        const parent = existing.parentNode;
        while (existing.firstChild) parent.insertBefore(existing.firstChild, existing);
        parent.removeChild(existing);
        parent.normalize();
        return;
      }
      if (sel.isCollapsed) return;
      const t = sel.toString();
      document.execCommand(
        "insertHTML",
        false,
        "<code>" + t.replace(/&/g, "&amp;").replace(/</g, "&lt;") + "</code>"
      );
    },

    setBlock(tag) {
      if (this.mode === "code") {
        if (tag === "blockquote") this._mdQuote();
        else this._mdBlock(tag);
        return;
      }
      this._restoreSel();
      // Clicking the active block type toggles back to a paragraph.
      const target = this.blockType === tag ? "p" : tag;
      document.execCommand("formatBlock", false, "<" + target + ">");
      this.$refs.visual.focus();
      this._refreshToolbar();
    },

    toggleList(listTag) {
      if (this.mode === "code") {
        this._mdList(listTag);
        return;
      }
      this._restoreSel();
      document.execCommand(listTag === "ul" ? "insertUnorderedList" : "insertOrderedList");
      this.$refs.visual.focus();
      this._refreshToolbar();
    },

    // Toggle code block on the current block element. Kept as DOM surgery:
    // formatBlock can't produce the <pre><code> pair turndown expects.
    setCodeBlock() {
      if (this.mode === "code") {
        this._mdCodeBlock();
        return;
      }
      this._restoreSel();
      const visual = this.$refs.visual;
      const sel = window.getSelection();
      if (!sel || !sel.rangeCount) return;

      let node = sel.getRangeAt(0).startContainer;
      if (node.nodeType === 3) node = node.parentNode;

      const pre = this._ancestor(node, visual, (n) => n.tagName === "PRE");
      if (pre) {
        const p = document.createElement("p");
        p.textContent = pre.textContent;
        pre.parentNode.replaceChild(p, pre);
        const r = document.createRange();
        r.selectNodeContents(p);
        r.collapse(false);
        sel.removeAllRanges();
        sel.addRange(r);
        visual.focus();
        this._refreshToolbar();
        return;
      }

      while (node.parentElement && node.parentElement !== visual) node = node.parentElement;
      if (!node || node.parentElement !== visual) return;

      const newPre = document.createElement("pre");
      newPre.className = "codeblock";
      const code = document.createElement("code");
      code.textContent = node.textContent;
      newPre.appendChild(code);
      node.parentNode.replaceChild(newPre, node);

      const r = document.createRange();
      r.selectNodeContents(code);
      r.collapse(false);
      sel.removeAllRanges();
      sel.addRange(r);
      this._ensureTrailingParagraph();
      visual.focus();
      this._refreshToolbar();
    },

    insertLink() {
      if (this.mode === "code") {
        this._mdLink();
        return;
      }
      const url = prompt("Link URL", "https://");
      if (!url) return;
      this._restoreSel();
      const sel = window.getSelection();
      if (sel && sel.isCollapsed) {
        document.execCommand("insertHTML", false, '<a href="' + escHTML(url) + '">' + escHTML(url) + "</a>");
      } else {
        document.execCommand("createLink", false, url);
      }
      this.$refs.visual.focus();
    },

    // Keyboard handling in visual mode. Everything is native except Enter
    // inside a code block: browsers split the <pre> into two blocks, so we
    // insert a <br> instead to keep it one fenced block.
    onVisualKeydown(e) {
      if (e.key !== "Enter" || e.shiftKey) return;
      const visual = this.$refs.visual;
      const sel = window.getSelection();
      if (!sel || !sel.rangeCount) return;
      const pre = this._ancestor(
        sel.getRangeAt(0).commonAncestorContainer,
        visual,
        (n) => n.tagName === "PRE"
      );
      if (!pre) return;
      e.preventDefault();
      const range = sel.getRangeAt(0);
      range.deleteContents();
      const br = document.createElement("br");
      range.insertNode(br);
      const r = document.createRange();
      r.setStartAfter(br);
      r.collapse(true);
      sel.removeAllRanges();
      sel.addRange(r);
    },

    // Code-mode formatting — the toolbar inserts raw markdown into the textarea.

    // Wrap the selection in an inline marker (** * ~~ `), or unwrap when the
    // selection (or the text just outside it) already carries that marker.
    _mdInline(marker) {
      const ta = this.$refs.codearea;
      if (!ta) return;
      const len = marker.length;
      const start = ta.selectionStart;
      const end = ta.selectionEnd;
      const val = ta.value;
      const sel = val.slice(start, end);
      // A single * must not strip one star off a ** pair.
      const star = marker === "*";

      if (
        sel.length >= 2 * len && sel.startsWith(marker) && sel.endsWith(marker) &&
        !(star && (sel[len] === "*" || sel[sel.length - len - 1] === "*"))
      ) {
        // Markers inside the selection: |**bold**| → |bold|
        ta.setRangeText(sel.slice(len, sel.length - len), start, end, "select");
      } else if (
        start >= len && val.slice(start - len, start) === marker && val.slice(end, end + len) === marker &&
        !(star && (val[start - len - 1] === "*" || val[end + len] === "*"))
      ) {
        // Markers just outside the selection: **|bold|** → |bold|
        ta.setRangeText(sel, start - len, end + len, "select");
      } else {
        // Wrap. With no selection, leave the cursor between the markers.
        ta.setRangeText(marker + sel + marker, start, end);
        ta.setSelectionRange(start + len, end + len);
      }
      this.src = ta.value;
      ta.focus();
    },

    // Set the heading level of the current line (# / ## / ###, "p" strips).
    _mdBlock(tag) {
      const ta = this.$refs.codearea;
      if (!ta) return;
      const val = ta.value;
      const lineStart = val.lastIndexOf("\n", ta.selectionStart - 1) + 1;
      let lineEnd = val.indexOf("\n", ta.selectionStart);
      if (lineEnd === -1) lineEnd = val.length;
      const line = val.slice(lineStart, lineEnd).replace(/^#{1,6}\s+/, "");
      // Clicking the active heading level strips it back to plain text.
      const cur = { h1: "# ", h2: "## ", h3: "### " }[tag] || "";
      const had = val.slice(lineStart, lineEnd).startsWith(cur) && cur !== "";
      const prefix = had ? "" : cur;
      ta.setRangeText(prefix + line, lineStart, lineEnd);
      const pos = lineStart + prefix.length + line.length;
      ta.setSelectionRange(pos, pos);
      this.src = ta.value;
      ta.focus();
    },

    // Toggle a list prefix (- or 1. 2. 3.) on every selected line.
    _mdList(listTag) {
      const ta = this.$refs.codearea;
      if (!ta) return;
      const val = ta.value;
      const blockStart = val.lastIndexOf("\n", ta.selectionStart - 1) + 1;
      let blockEnd = val.indexOf("\n", Math.max(ta.selectionEnd - 1, ta.selectionStart));
      if (blockEnd === -1) blockEnd = val.length;
      const lines = val.slice(blockStart, blockEnd).split("\n");
      const re = listTag === "ul" ? /^\s*- / : /^\s*\d+\. /;
      const allListed = lines.every((l) => !l.trim() || re.test(l));
      const out = lines.map((l, i) => {
        if (!l.trim()) return l;
        const bare = l.replace(/^\s*(?:- |\d+\. )/, "");
        if (allListed) return bare;
        return (listTag === "ul" ? "- " : `${i + 1}. `) + bare;
      }).join("\n");
      ta.setRangeText(out, blockStart, blockEnd, "select");
      this.src = ta.value;
      ta.focus();
    },

    // Toggle a "> " quote prefix on every selected line.
    _mdQuote() {
      const ta = this.$refs.codearea;
      if (!ta) return;
      const val = ta.value;
      const blockStart = val.lastIndexOf("\n", ta.selectionStart - 1) + 1;
      let blockEnd = val.indexOf("\n", Math.max(ta.selectionEnd - 1, ta.selectionStart));
      if (blockEnd === -1) blockEnd = val.length;
      const lines = val.slice(blockStart, blockEnd).split("\n");
      const allQuoted = lines.every((l) => !l.trim() || /^\s*> /.test(l));
      const out = lines.map((l) => {
        if (!l.trim()) return l;
        return allQuoted ? l.replace(/^\s*> /, "") : "> " + l;
      }).join("\n");
      ta.setRangeText(out, blockStart, blockEnd, "select");
      this.src = ta.value;
      ta.focus();
    },

    // Wrap the selection in [text](url); the label stays selected for editing.
    _mdLink() {
      const ta = this.$refs.codearea;
      if (!ta) return;
      const url = prompt("Link URL", "https://");
      if (!url) {
        ta.focus();
        return;
      }
      const start = ta.selectionStart;
      const end = ta.selectionEnd;
      const sel = ta.value.slice(start, end) || "text";
      ta.setRangeText("[" + sel + "](" + url + ")", start, end);
      ta.setSelectionRange(start + 1, start + 1 + sel.length);
      this.src = ta.value;
      ta.focus();
    },

    // Wrap the selected lines in ``` fences, or strip surrounding fence lines.
    _mdCodeBlock() {
      const ta = this.$refs.codearea;
      if (!ta) return;
      const val = ta.value;
      const blockStart = val.lastIndexOf("\n", ta.selectionStart - 1) + 1;
      let blockEnd = val.indexOf("\n", Math.max(ta.selectionEnd - 1, ta.selectionStart));
      if (blockEnd === -1) blockEnd = val.length;
      const block = val.slice(blockStart, blockEnd);
      const lines = block.split("\n");
      if (lines.length >= 2 && /^```/.test(lines[0]) && /^```\s*$/.test(lines[lines.length - 1])) {
        ta.setRangeText(lines.slice(1, -1).join("\n"), blockStart, blockEnd, "select");
      } else {
        ta.setRangeText("```\n" + block + "\n```", blockStart, blockEnd, "select");
      }
      this.src = ta.value;
      ta.focus();
    },

    // Mention handling in visual mode

    // Fired by @input on the contenteditable in visual mode.
    onVisualInput() {
      const sel = window.getSelection();
      if (!sel || !sel.rangeCount || !this.$refs.visual) return;

      const range = sel.getRangeAt(0);
      const preRange = document.createRange();
      preRange.selectNodeContents(this.$refs.visual);
      preRange.setEnd(range.startContainer, range.startOffset);
      const textBefore = preRange.toString();

      const mUser = textBefore.match(/@(\w+)$/);
      const mTicket = textBefore.match(/#([\w-]+)$/);

      if (mUser) {
        this.ticketOpen = false;
        this._mentionFetch(mUser[1]);
      } else if (mTicket) {
        this.mentionOpen = false;
        this._ticketFetch(mTicket[1]);
      } else {
        this.mentionOpen = false;
        this.ticketOpen = false;
      }
    },

    // Override mentionPick/ticketPick to route through visual-mode insertion.
    mentionPick(username) {
      if (this.mode === "visual") {
        this._visualInsert("@" + username + " ", /@\w*$/);
      } else {
        const ta = this._mentionTa;
        if (!ta) return;
        const before = ta.value.slice(0, ta.selectionStart);
        const after = ta.value.slice(ta.selectionStart);
        const nb = before.replace(/@\w+$/, "@" + username + " ");
        ta.value = nb + after;
        ta.setSelectionRange(nb.length, nb.length);
        ta.focus();
        this.mentionOpen = false;
      }
    },

    ticketPick(displayID) {
      if (this.mode === "visual") {
        this._visualInsert("#" + displayID + " ", /#[\w-]*$/);
      } else {
        const ta = this._mentionTa;
        if (!ta) return;
        const before = ta.value.slice(0, ta.selectionStart);
        const after = ta.value.slice(ta.selectionStart);
        const nb = before.replace(/#[\w-]+$/, "#" + displayID + " ");
        ta.value = nb + after;
        ta.setSelectionRange(nb.length, nb.length);
        ta.focus();
        this.ticketOpen = false;
      }
    },

    // Replace the typed trigger text at the cursor with `text`.
    _visualInsert(text, triggerRe) {
      if (!this._lastRange || !this.$refs.visual) return;
      const sel = window.getSelection();
      if (!sel) return;

      sel.removeAllRanges();
      sel.addRange(this._lastRange.cloneRange());

      const range = sel.getRangeAt(0);
      const node = range.startContainer;
      const offset = range.startOffset;

      // Only works when the trigger text is in the same text node as the cursor,
      // which is always the case when the user is actively typing.
      if (node.nodeType !== Node.TEXT_NODE) return;

      const textBefore = node.textContent.slice(0, offset);
      const match = textBefore.match(triggerRe);
      if (!match) return;

      const delRange = document.createRange();
      delRange.setStart(node, offset - match[0].length);
      delRange.setEnd(node, offset);
      delRange.deleteContents();

      const textNode = document.createTextNode(text);
      delRange.insertNode(textNode);

      const cursor = document.createRange();
      cursor.setStartAfter(textNode);
      cursor.collapse(true);
      sel.removeAllRanges();
      sel.addRange(cursor);

      this.$refs.visual.focus();
      this.mentionOpen = false;
      this.ticketOpen = false;
    },

    // Helpers

    // Ensure the visual pane always ends with an empty paragraph so the user
    // can click below any heading or code block and type normally.
    _ensureTrailingParagraph() {
      const visual = this.$refs.visual;
      if (!visual) return;
      const last = visual.lastElementChild;
      if (last && last.tagName === "P" && !last.textContent.trim()) return;
      const p = document.createElement("p");
      p.innerHTML = "<br>";
      visual.appendChild(p);
    },

    // Remove the sentinel trailing paragraph before converting to markdown so
    // it doesn't produce a spurious blank line at the end.
    _stripTrailingParagraph() {
      const visual = this.$refs.visual;
      if (!visual) return;
      const last = visual.lastElementChild;
      if (last && last.tagName === "P" && !last.textContent.trim()) last.remove();
    },

    _restoreSel() {
      if (!this._lastRange) return;
      const sel = window.getSelection();
      if (!sel) return;
      sel.removeAllRanges();
      sel.addRange(this._lastRange.cloneRange());
    },

    _ancestor(node, boundary, test) {
      let cur = node && node.nodeType === 3 ? node.parentNode : node;
      while (cur && cur !== boundary) {
        if (cur.nodeType === 1 && test(cur)) return cur;
        cur = cur.parentNode;
      }
      return null;
    },
  };
}

window.richEditor = richEditor;
