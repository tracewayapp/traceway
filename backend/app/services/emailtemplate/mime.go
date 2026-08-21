package emailtemplate

import (
	"bytes"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"strings"
	"time"
)

// BuildMIME composes a multipart/alternative message carrying both the
// plaintext and the rendered HTML body, quoted-printable encoded so long
// stack-trace and markup lines stay within SMTP line limits. An empty
// htmlBody yields a plain single-part message.
func BuildMIME(from string, to []string, subject, textBody, htmlBody string) ([]byte, error) {
	var buf bytes.Buffer

	fmt.Fprintf(&buf, "From: %s\r\n", from)
	fmt.Fprintf(&buf, "To: %s\r\n", strings.Join(to, ", "))
	fmt.Fprintf(&buf, "Subject: %s\r\n", mime.QEncoding.Encode("utf-8", subject))
	fmt.Fprintf(&buf, "Date: %s\r\n", time.Now().Format(time.RFC1123Z))
	buf.WriteString("MIME-Version: 1.0\r\n")

	if htmlBody == "" {
		buf.WriteString("Content-Type: text/plain; charset=utf-8\r\n")
		buf.WriteString("Content-Transfer-Encoding: quoted-printable\r\n\r\n")
		if err := writeQuotedPrintable(&buf, textBody); err != nil {
			return nil, err
		}
		return buf.Bytes(), nil
	}

	mw := multipart.NewWriter(&buf)
	fmt.Fprintf(&buf, "Content-Type: multipart/alternative; boundary=%q\r\n\r\n", mw.Boundary())

	for _, part := range []struct {
		contentType string
		body        string
	}{
		{"text/plain; charset=utf-8", textBody},
		{"text/html; charset=utf-8", htmlBody},
	} {
		pw, err := mw.CreatePart(map[string][]string{
			"Content-Type":              {part.contentType},
			"Content-Transfer-Encoding": {"quoted-printable"},
		})
		if err != nil {
			return nil, err
		}
		if err := writeQuotedPrintable(pw, part.body); err != nil {
			return nil, err
		}
	}
	if err := mw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func writeQuotedPrintable(w io.Writer, body string) error {
	qp := quotedprintable.NewWriter(w)
	if _, err := qp.Write([]byte(body)); err != nil {
		return err
	}
	return qp.Close()
}
