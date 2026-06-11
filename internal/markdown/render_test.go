package markdown

import (
	"strings"
	"testing"
)

func TestRenderBasicMarkdown(t *testing.T) {
	out := string(Render("**bold** and *italic*"))
	if !strings.Contains(out, "<strong>bold</strong>") {
		t.Errorf("bold not rendered: %s", out)
	}
	if !strings.Contains(out, "<em>italic</em>") {
		t.Errorf("italic not rendered: %s", out)
	}
}

func TestRenderEmptyInput(t *testing.T) {
	if out := Render(""); out != "" {
		t.Errorf("empty input should render empty, got %q", out)
	}
}

func TestRenderUserMention(t *testing.T) {
	out := string(Render("ping @alice about this"))
	if !strings.Contains(out, `href="/users/alice"`) {
		t.Errorf("user mention not linked: %s", out)
	}
	if !strings.Contains(out, "@alice") {
		t.Errorf("mention text missing: %s", out)
	}
}

func TestRenderTicketReference(t *testing.T) {
	out := string(Render("blocked by #BE-42 for now"))
	if !strings.Contains(out, `href="/tickets/BE-42"`) {
		t.Errorf("ticket ref not linked: %s", out)
	}
}

func TestRenderMentionInsideCodeFenceIsSkipped(t *testing.T) {
	src := "```\n@alice #BE-42\n```"
	out := string(Render(src))
	if strings.Contains(out, "/users/alice") || strings.Contains(out, "/tickets/BE-42") {
		t.Errorf("mentions inside code fence must not become links: %s", out)
	}
}

func TestRenderStripsScript(t *testing.T) {
	out := string(Render(`hello <script>alert("x")</script> world`))
	if strings.Contains(out, "<script") {
		t.Errorf("script tag survived sanitisation: %s", out)
	}
}

func TestRenderStripsExternalEventHandlers(t *testing.T) {
	out := string(Render(`<a href="https://example.com" onclick="evil()">link</a>`))
	if strings.Contains(out, "onclick") {
		t.Errorf("onclick attribute survived sanitisation: %s", out)
	}
}

func TestRenderTaskList(t *testing.T) {
	out := string(Render("- [ ] open item\n- [x] done item"))
	if !strings.Contains(out, `type="checkbox"`) {
		t.Errorf("task list checkboxes not rendered: %s", out)
	}
	if !strings.Contains(out, "checked") {
		t.Errorf("checked state missing: %s", out)
	}
}

func TestRenderEmailNotLinkedAsMention(t *testing.T) {
	// The @ in an email address follows a word character, not whitespace,
	// so it must not be treated as a user mention.
	out := string(Render("mail me at someone@example.com"))
	if strings.Contains(out, "/users/example") {
		t.Errorf("email domain wrongly linked as mention: %s", out)
	}
}
