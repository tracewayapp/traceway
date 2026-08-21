package emailtemplate

import (
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"strings"
	"testing"
)

func TestRenderEscapesAndIncludesContent(t *testing.T) {
	html, err := Render(Data{
		LogoURL:    "https://traceway.example.com/traceway-mark.png",
		Title:      "[Project <X>] New error: *traceway.StackTraceError",
		Badge:      "CRITICAL",
		BadgeColor: ColorCritical,
		Paragraphs: []string{"A new error has been detected.\nSecond line."},
		Details:    []Detail{{Label: "Hash", Value: "9c7bc73da37328f2"}},
		CodeBlock:  "err := <nil>\n\tat main.go:1",
		Button:     &Button{Label: "View Issue", URL: "https://traceway.example.com/issues/abc"},
		FooterNote: "You are receiving this because a rule fired.",
	})
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	for _, want := range []string{
		"[Project &lt;X&gt;] New error: *traceway.StackTraceError",
		"CRITICAL",
		"A new error has been detected.<br>Second line.",
		"9c7bc73da37328f2",
		"err := &lt;nil&gt;",
		`href="https://traceway.example.com/issues/abc"`,
		"View Issue",
		"Button not working?",
		"You are receiving this because a rule fired.",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("rendered HTML missing %q", want)
		}
	}
}

func TestRenderButtonIsOutlineStyle(t *testing.T) {
	html, err := Render(Data{Title: "t", Button: &Button{Label: "Go", URL: "https://x"}})
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	if !strings.Contains(html, "border:1px solid #d1d9e0;border-radius:6px;background-color:#ffffff;") {
		t.Error("expected the outline button style")
	}
}

func TestBuildMIMEMultipart(t *testing.T) {
	raw, err := BuildMIME("noreply@traceway.example.com", []string{"dev@example.com"},
		"[CRITICAL] New error: über long subject", "plain text body", "<html><body>hi</body></html>")
	if err != nil {
		t.Fatalf("BuildMIME failed: %v", err)
	}

	msg, err := mail.ReadMessage(strings.NewReader(string(raw)))
	if err != nil {
		t.Fatalf("message does not parse: %v", err)
	}

	dec := new(mime.WordDecoder)
	subject, err := dec.DecodeHeader(msg.Header.Get("Subject"))
	if err != nil {
		t.Fatalf("subject does not decode: %v", err)
	}
	if subject != "[CRITICAL] New error: über long subject" {
		t.Errorf("unexpected subject: %q", subject)
	}

	mediaType, params, err := mime.ParseMediaType(msg.Header.Get("Content-Type"))
	if err != nil || mediaType != "multipart/alternative" {
		t.Fatalf("unexpected content type %q (err %v)", mediaType, err)
	}

	mr := multipart.NewReader(msg.Body, params["boundary"])
	var bodies []string
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("reading part: %v", err)
		}
		body, err := io.ReadAll(quotedprintable.NewReader(part))
		if err != nil {
			t.Fatalf("decoding part: %v", err)
		}
		bodies = append(bodies, string(body))
	}
	if len(bodies) != 2 || bodies[0] != "plain text body" || bodies[1] != "<html><body>hi</body></html>" {
		t.Errorf("unexpected part bodies: %#v", bodies)
	}
}

func TestBuildMIMEPlainOnly(t *testing.T) {
	raw, err := BuildMIME("a@b.c", []string{"d@e.f"}, "subject", "just text", "")
	if err != nil {
		t.Fatalf("BuildMIME failed: %v", err)
	}
	msg, err := mail.ReadMessage(strings.NewReader(string(raw)))
	if err != nil {
		t.Fatalf("message does not parse: %v", err)
	}
	body, err := io.ReadAll(quotedprintable.NewReader(msg.Body))
	if err != nil {
		t.Fatalf("decoding body: %v", err)
	}
	if string(body) != "just text" {
		t.Errorf("unexpected body: %q", body)
	}
}
