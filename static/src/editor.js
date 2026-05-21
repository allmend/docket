import { marked } from "marked";
import TurndownService from "turndown";

marked.use({ breaks: true, gfm: true });

const td = new TurndownService({
  headingStyle: "atx",
  bulletListMarker: "-",
  codeBlockStyle: "fenced",
  emDelimiter: "*",
});

td.addRule("strikethrough", {
  filter: (node) =>
    node.nodeName === "DEL" || node.nodeName === "S" || node.nodeName === "STRIKE",
  replacement: (content) => `~~${content}~~`,
});

// Fenced code blocks may contain <br> elements (inserted by contenteditable
// when the user presses Enter inside a <pre>). Walk child nodes explicitly so
// both text-node newlines (from marked.parse) and <br> elements are preserved.
td.addRule("fenced-code-block", {
  filter: (node) =>
    node.nodeName === "PRE" && node.firstChild && node.firstChild.nodeName === "CODE",
  replacement: (_content, node) => {
    const code = node.firstChild;
    // marked sets class="language-javascript" — preserve the lang identifier.
    const lang = (code.className || "").replace(/\blanguage-/, "").trim();
    let text = "";
    code.childNodes.forEach((child) => {
      if (child.nodeType === 3) text += child.textContent;
      else if (child.nodeName === "BR") text += "\n";
    });
    return "\n\n```" + lang + "\n" + text.replace(/\n$/, "") + "\n```\n\n";
  },
});

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
              this.$refs.codearea.value = td.turndown(this.$refs.visual.innerHTML.replace(/​/g, "") || "");
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
        this._updateToolbar(sel, container);
      };
      document.addEventListener("selectionchange", onSel);
      this._cleanups.push(() => document.removeEventListener("selectionchange", onSel));
    },

    destroy() {
      this._cleanups.forEach((fn) => fn());
    },

    // Mode switching

    switchToVisual() {
      const h = this.$refs.codearea ? this.$refs.codearea.offsetHeight : 0;
      if (this.$refs.codearea) this.src = this.$refs.codearea.value;
      this.mode = "visual";
      localStorage.setItem(this._STORAGE_KEY, "visual");
      this.$nextTick(() => {
        if (!this.$refs.visual) return;
        if (h > 0) this.$refs.visual.style.minHeight = h + "px";
        this.$refs.visual.innerHTML = marked.parse(this.src || "");
        this._ensureTrailingParagraph();
        this.$refs.visual.focus();
      });
    },

    switchToCode() {
      const h = this.$refs.visual ? this.$refs.visual.offsetHeight : 0;
      if (this.$refs.visual) {
        this._stripTrailingParagraph();
        this.src = td.turndown(this.$refs.visual.innerHTML.replace(/​/g, "") || "");
      }
      this.mode = "code";
      localStorage.setItem(this._STORAGE_KEY, "code");
      this.$nextTick(() => {
        if (!this.$refs.codearea) return;
        if (h > 0) this.$refs.codearea.style.minHeight = h + "px";
        this.$refs.codearea.value = this.src;
        this.$refs.codearea.focus();
      });
    },

    // Toolbar state

    _updateToolbar(sel, container) {
      const v = this.$refs.visual;
      this.isBold = !!this._ancestor(container, v, (n) => n.tagName === "STRONG" || n.tagName === "B");
      this.isItalic = !!this._ancestor(container, v, (n) => n.tagName === "EM" || n.tagName === "I");
      this.isStrike = !!this._ancestor(container, v, (n) =>
        ["S", "DEL", "STRIKE"].includes(n.tagName)
      );
      this.isCode = !!this._ancestor(container, v, (n) => n.tagName === "CODE");
      const block = this._ancestor(container, v, (n) =>
        ["P", "H1", "H2", "H3", "H4", "H5", "H6", "LI", "BLOCKQUOTE"].includes(n.tagName)
      );
      this.blockType = block ? block.tagName.toLowerCase() : "p";
    },

    // Formatting actions

    toggleInline(tag) {
      this._restoreSel();
      const visual = this.$refs.visual;
      const sel = window.getSelection();
      if (!sel || !sel.rangeCount) return;
      const range = sel.getRangeAt(0);

      const existing = this._ancestor(
        range.commonAncestorContainer,
        visual,
        (n) => n.tagName === tag.toUpperCase()
      );

      if (existing) {
        const parent = existing.parentNode;
        while (existing.firstChild) parent.insertBefore(existing.firstChild, existing);
        parent.removeChild(existing);
        parent.normalize();
      } else if (!sel.isCollapsed) {
        const el = document.createElement(tag);
        try {
          range.surroundContents(el);
        } catch (_) {
          const frag = range.extractContents();
          el.appendChild(frag);
          range.insertNode(el);
          const r = document.createRange();
          r.selectNodeContents(el);
          sel.removeAllRanges();
          sel.addRange(r);
        }
        // Place a space after the inline element so the cursor exits the style.
        const space = document.createTextNode(" ");
        el.parentNode.insertBefore(space, el.nextSibling);
        const r = document.createRange();
        r.setStart(space, 1);
        r.collapse(true);
        sel.removeAllRanges();
        sel.addRange(r);
      }
      visual.focus();
    },

    setBlock(tag) {
      this._restoreSel();
      const visual = this.$refs.visual;
      const sel = window.getSelection();
      if (!sel || !sel.rangeCount) return;

      let node = sel.getRangeAt(0).startContainer;
      if (node.nodeType === 3) node = node.parentNode;
      while (node.parentElement && node.parentElement !== visual) {
        node = node.parentElement;
      }
      if (!node || node.parentElement !== visual) return;

      const targetTag = node.tagName.toLowerCase() === tag ? "p" : tag;
      const newEl = document.createElement(targetTag);
      newEl.innerHTML = node.innerHTML;
      node.parentNode.replaceChild(newEl, node);

      const r = document.createRange();
      r.selectNodeContents(newEl);
      r.collapse(false);
      sel.removeAllRanges();
      sel.addRange(r);
      this._ensureTrailingParagraph();
      visual.focus();
    },

    toggleList(listTag) {
      this._restoreSel();
      const visual = this.$refs.visual;
      const sel = window.getSelection();
      if (!sel || !sel.rangeCount) return;

      const listEl = this._ancestor(
        sel.getRangeAt(0).commonAncestorContainer,
        visual,
        (n) => n.tagName === "UL" || n.tagName === "OL"
      );

      if (listEl) {
        if (listEl.tagName === listTag.toUpperCase()) {
          const parent = listEl.parentNode;
          Array.from(listEl.querySelectorAll("li")).forEach((li) => {
            const p = document.createElement("p");
            p.innerHTML = li.innerHTML;
            parent.insertBefore(p, listEl);
          });
          parent.removeChild(listEl);
        } else {
          const newList = document.createElement(listTag);
          newList.innerHTML = listEl.innerHTML;
          listEl.parentNode.replaceChild(newList, listEl);
        }
        visual.focus();
        return;
      }

      let node = sel.getRangeAt(0).startContainer;
      if (node.nodeType === 3) node = node.parentNode;
      while (node.parentElement && node.parentElement !== visual) {
        node = node.parentElement;
      }
      if (!node || node.parentElement !== visual) return;

      const list = document.createElement(listTag);
      const li = document.createElement("li");
      li.innerHTML = node.innerHTML;
      list.appendChild(li);
      node.parentNode.replaceChild(list, node);

      const r = document.createRange();
      r.selectNodeContents(li);
      r.collapse(false);
      sel.removeAllRanges();
      sel.addRange(r);
      visual.focus();
    },

    // Keyboard handling in visual mode

    onVisualKeydown(e) {
      const visual = this.$refs.visual;
      const sel = window.getSelection();
      if (!sel || !sel.rangeCount) return;

      // Enter in visual mode: handled entirely by us so the cursor always lands
      // unambiguously at the start of the new line via a text-node anchor.
      if (e.key === "Enter") {
        const anchor = sel.getRangeAt(0).commonAncestorContainer;

        // Code block: insert a <br> element. Text-node \n is unreliable in
        // contenteditable — browsers use <br> for editable line breaks in <pre>.
        const pre = this._ancestor(anchor, visual, (n) => n.tagName === "PRE");
        if (pre) {
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
          return;
        }

        // List item: let the browser create the next <li> natively.
        const li = this._ancestor(anchor, visual, (n) => n.tagName === "LI");
        if (li) return;

        // All other blocks: split at cursor into two paragraphs.
        e.preventDefault();
        const range = sel.getRangeAt(0);
        range.deleteContents();

        let block = anchor.nodeType === 3 ? anchor.parentNode : anchor;
        while (block.parentElement && block.parentElement !== visual) {
          block = block.parentElement;
        }
        if (!block || block.parentElement !== visual) return;

        const newP = document.createElement("p");
        const isHeading = ["H1","H2","H3","H4","H5","H6"].includes(block.tagName);

        if (!isHeading) {
          // Extract everything from cursor to end of block into the new paragraph.
          const afterRange = document.createRange();
          afterRange.setStart(range.startContainer, range.startOffset);
          afterRange.setEnd(block, block.childNodes.length);
          const frag = afterRange.extractContents();
          if (frag.childNodes.length) newP.appendChild(frag);
          // If the old block is now empty, give it a text anchor for height.
          if (!block.textContent) {
            block.appendChild(document.createTextNode("​"));
          }
        }

        // New paragraph always gets a text-node anchor so the browser renders
        // the caret unambiguously on the new line, not at the end of the old one.
        if (!newP.textContent) {
          newP.appendChild(document.createTextNode("​"));
        }

        block.parentNode.insertBefore(newP, block.nextSibling);

        const walker = document.createTreeWalker(newP, NodeFilter.SHOW_TEXT);
        const firstText = walker.nextNode();
        const r = document.createRange();
        r.setStart(firstText || newP, 0);
        r.collapse(true);
        sel.removeAllRanges();
        sel.addRange(r);
        return;
      }

      // ArrowRight at the end of an inline element → jump cursor out of it.
      if (e.key === "ArrowRight" && !e.shiftKey) {
        const range = sel.getRangeAt(0);
        if (!range.collapsed) return;
        const node = range.startContainer;
        if (node.nodeType !== Node.TEXT_NODE || range.startOffset !== node.length) return;
        const inlineEl = this._ancestor(
          node, visual,
          (n) => ["STRONG", "EM", "S", "CODE", "B", "I"].includes(n.tagName)
        );
        if (!inlineEl) return;
        e.preventDefault();
        const r = document.createRange();
        r.setStartAfter(inlineEl);
        r.collapse(true);
        sel.removeAllRanges();
        sel.addRange(r);
        return;
      }

      // Inline markdown shortcuts — backtick, asterisk, tilde, space (for heading prefixes).
      this._tryMarkdownShortcut(e, sel);
    },

    _tryMarkdownShortcut(e, sel) {
      if (!["` ", "*", "~", " "].includes(e.key) && e.key !== "`") return;
      const range = sel.getRangeAt(0);
      if (!range.collapsed) return;
      const node = range.startContainer;
      if (node.nodeType !== Node.TEXT_NODE) return;
      const visual = this.$refs.visual;
      const offset = range.startOffset;
      const before = node.textContent.slice(0, offset);

      // ``` at start of block → code block.
      if (e.key === "`" && before === "``") {
        e.preventDefault();
        node.textContent = node.textContent.slice(offset);
        let block = node.nodeType === 3 ? node.parentNode : node;
        while (block.parentElement && block.parentElement !== visual) block = block.parentElement;
        if (!block || block.parentElement !== visual) return;
        const pre = document.createElement("pre");
        const code = document.createElement("code");
        pre.appendChild(code);
        block.parentNode.replaceChild(pre, block);
        const r = document.createRange();
        r.selectNodeContents(code);
        r.collapse(false);
        sel.removeAllRanges();
        sel.addRange(r);
        this._ensureTrailingParagraph();
        visual.focus();
        return;
      }

      // `inline code`
      if (e.key === "`") {
        if (this._ancestor(node, visual, (n) => n.tagName === "CODE" || n.tagName === "PRE")) return;
        const idx = before.lastIndexOf("`");
        if (idx === -1 || idx === offset - 1) return;
        const content = before.slice(idx + 1);
        if (!content.trim()) return;
        e.preventDefault();
        this._applyShortcut(node, offset, idx, content, "code");
        return;
      }

      // **bold**  (closing second *)
      if (e.key === "*" && before.endsWith("*")) {
        const searchIn = before.slice(0, -1);
        const idx = searchIn.lastIndexOf("**");
        if (idx !== -1) {
          const content = searchIn.slice(idx + 2);
          if (content.trim() && !content.includes("*")) {
            e.preventDefault();
            // before = "prefix **content*" — openIdx points at first * of **
            this._applyShortcut(node, offset, idx, content, "strong");
            return;
          }
        }
      }

      // *italic*  (single *)
      if (e.key === "*") {
        const idx = before.lastIndexOf("*");
        if (idx === -1) return;
        if (idx > 0 && before[idx - 1] === "*") return; // part of **
        const content = before.slice(idx + 1);
        if (!content.trim() || content.includes("*")) return;
        e.preventDefault();
        this._applyShortcut(node, offset, idx, content, "em");
        return;
      }

      // ~~strikethrough~~  (closing second ~)
      if (e.key === "~" && before.endsWith("~")) {
        const searchIn = before.slice(0, -1);
        const idx = searchIn.lastIndexOf("~~");
        if (idx !== -1) {
          const content = searchIn.slice(idx + 2);
          if (content.trim() && !content.includes("~")) {
            e.preventDefault();
            this._applyShortcut(node, offset, idx, content, "s");
            return;
          }
        }
      }

      // Space after # / ## / ### at start of block → heading.
      if (e.key === " " && (before === "#" || before === "##" || before === "###")) {
        const tag = before === "#" ? "h1" : before === "##" ? "h2" : "h3";
        e.preventDefault();
        node.textContent = node.textContent.slice(offset);
        let block = node.nodeType === 3 ? node.parentNode : node;
        while (block.parentElement && block.parentElement !== visual) block = block.parentElement;
        if (!block || block.parentElement !== visual) return;
        const heading = document.createElement(tag);
        heading.innerHTML = block.innerHTML || "<br>";
        block.parentNode.replaceChild(heading, block);
        const r = document.createRange();
        r.selectNodeContents(heading);
        r.collapse(false);
        sel.removeAllRanges();
        sel.addRange(r);
        this._ensureTrailingParagraph();
        visual.focus();
      }
    },

    // Replace a raw markdown span in a text node with a formatted inline element.
    _applyShortcut(node, offset, openIdx, content, tag) {
      const sel = window.getSelection();
      const visual = this.$refs.visual;
      const prefix = node.textContent.slice(0, openIdx);
      const after = node.textContent.slice(offset);

      const el = document.createElement(tag);
      el.textContent = content;
      const space = document.createTextNode(" ");

      node.textContent = prefix;
      const parent = node.parentNode;
      const next = node.nextSibling;
      parent.insertBefore(el, next);
      parent.insertBefore(space, el.nextSibling);
      if (after) parent.insertBefore(document.createTextNode(after), space.nextSibling);

      const r = document.createRange();
      r.setStart(space, 1);
      r.collapse(true);
      sel.removeAllRanges();
      sel.addRange(r);
      visual.focus();
    },

    // Toggle code block on the current block element.
    setCodeBlock() {
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
        return;
      }

      while (node.parentElement && node.parentElement !== visual) node = node.parentElement;
      if (!node || node.parentElement !== visual) return;

      const newPre = document.createElement("pre");
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
    },

    // Mention handling in visual mode

    // Fired by @input on the contenteditable in visual mode.
    onVisualInput() {
      const sel = window.getSelection();
      if (!sel || !sel.rangeCount || !this.$refs.visual) return;

      // Remove ZWS cursor anchors left by Enter handling once the user types.
      const range = sel.getRangeAt(0);
      const cn = range.startContainer;
      if (cn.nodeType === Node.TEXT_NODE && cn.textContent.includes("​")) {
        const offset = range.startOffset;
        const zwsIdx = cn.textContent.indexOf("​");
        cn.textContent = cn.textContent.replace("​", "");
        // Only shift the cursor left if the ZWS was strictly before it;
        // if ZWS was at or after the cursor, the position is already correct.
        const newOffset = zwsIdx < offset ? Math.max(0, offset - 1) : offset;
        const r = document.createRange();
        r.setStart(cn, newOffset);
        r.collapse(true);
        sel.removeAllRanges();
        sel.addRange(r);
      }
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
    // can click below any heading or inline-formatted text and type normally.
    _ensureTrailingParagraph() {
      const visual = this.$refs.visual;
      if (!visual) return;
      const last = visual.lastElementChild;
      if (last && last.tagName === "P" && !last.textContent.replace(/​/g, "").trim()) return;
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
