package notifications

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"time"

	"github.com/tracewayapp/traceway/backend/app/config"
	"github.com/tracewayapp/traceway/backend/app/services"
	"github.com/tracewayapp/traceway/backend/app/services/emailtemplate"
)

const smtpTimeout = 10 * time.Second

type EmailAdapter struct {
	Recipients []string `json:"recipients"`
}

func (a *EmailAdapter) Type() string { return "email" }

func (a *EmailAdapter) Validate() error {
	if len(a.Recipients) == 0 {
		return fmt.Errorf("at least one recipient is required")
	}
	if len(a.Recipients) > 10 {
		return fmt.Errorf("maximum 10 recipients allowed")
	}
	for _, r := range a.Recipients {
		if !strings.Contains(r, "@") {
			return fmt.Errorf("invalid email address: %s", r)
		}
	}
	return nil
}

func (a *EmailAdapter) Send(ctx context.Context, msg Message) error {
	emailSvc := services.EmailService
	if emailSvc == nil {
		return fmt.Errorf("email service not initialized")
	}

	prefix := ""
	switch msg.Severity {
	case SeverityCritical:
		prefix = "[CRITICAL] "
	case SeverityWarning:
		prefix = "[WARNING] "
	case SeverityInfo:
		prefix = "[INFO] "
	}

	subject := prefix + msg.Subject

	if !emailSvc.IsEnabled() {
		config.Logf("[EMAIL LOG] To: %s\nSubject: %s\nBody:\n%s", strings.Join(a.Recipients, ", "), subject, msg.Body)
		return nil
	}

	cfg := config.Config
	from := cfg.SMTPFrom
	// No username means an auth-less relay; smtp.PlainAuth would refuse to
	// run over an unencrypted connection anyway.
	var auth smtp.Auth
	if cfg.SMTPUsername != "" {
		auth = smtp.PlainAuth("", cfg.SMTPUsername, cfg.SMTPPassword, cfg.SMTPHost)
	}
	addr := fmt.Sprintf("%s:%s", cfg.SMTPHost, cfg.SMTPPort)

	htmlBody := msg.HTMLBody
	if htmlBody == "" {
		var err error
		htmlBody, err = renderMessageHTML(msg, emailSvc.BaseURL())
		if err != nil {
			return fmt.Errorf("failed to render email HTML: %w", err)
		}
	}
	emailMsg, err := emailtemplate.BuildMIME(from, a.Recipients, subject, msg.Body, htmlBody)
	if err != nil {
		return fmt.Errorf("failed to build email message: %w", err)
	}

	return sendMailWithTimeout(ctx, addr, auth, from, a.Recipients, emailMsg)
}

// renderMessageHTML renders any notification Message through the shared email
// layout. Structured fields (Intro/Details/CodeBlock) win over the plaintext
// Body, which stays the source for every text-only adapter.
func renderMessageHTML(msg Message, baseURL string) (string, error) {
	d := emailtemplate.Data{
		LogoURL: emailtemplate.LogoURL(baseURL),
		Title:   msg.Subject,
	}

	switch msg.Severity {
	case SeverityCritical:
		d.Badge, d.BadgeColor = "CRITICAL", emailtemplate.ColorCritical
	case SeverityWarning:
		d.Badge, d.BadgeColor = "WARNING", emailtemplate.ColorWarning
	case SeverityInfo:
		d.Badge, d.BadgeColor = "INFO", emailtemplate.ColorInfo
	}

	if msg.Intro != "" || len(msg.Details) > 0 || msg.CodeBlock != "" {
		if msg.Intro != "" {
			d.Paragraphs = []string{msg.Intro}
		}
		for _, det := range msg.Details {
			d.Details = append(d.Details, emailtemplate.Detail{Label: det.Label, Value: det.Value})
		}
		d.CodeBlock = msg.CodeBlock
	} else if body := strings.TrimSpace(msg.Body); body != "" {
		d.Paragraphs = strings.Split(body, "\n\n")
	}

	if msg.URL != "" {
		u := msg.URL
		if strings.HasPrefix(u, "/") {
			u = strings.TrimRight(baseURL, "/") + u
		}
		label := msg.ActionLabel
		if label == "" {
			label = "View in Traceway"
		}
		d.Button = &emailtemplate.Button{Label: label, URL: u}
	}

	if msg.RuleName != "" {
		d.FooterNote = fmt.Sprintf("You are receiving this because the notification rule %q fired.", msg.RuleName)
	}

	return emailtemplate.Render(d)
}

func sendMailWithTimeout(ctx context.Context, addr string, auth smtp.Auth, from string, to []string, msg []byte) error {
	dialer := net.Dialer{Timeout: smtpTimeout}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("SMTP dial failed: %w", err)
	}

	// Must be set before smtp.NewClient, which blocks reading the server greeting.
	deadline := time.Now().Add(smtpTimeout)
	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		deadline = ctxDeadline
	}
	conn.SetDeadline(deadline)

	host, _, _ := net.SplitHostPort(addr)
	client, err := smtp.NewClient(conn, host)
	if err != nil {
		conn.Close()
		return fmt.Errorf("SMTP client failed: %w", err)
	}
	defer client.Close()

	if ok, _ := client.Extension("STARTTLS"); ok {
		if err := client.StartTLS(&tls.Config{ServerName: host}); err != nil {
			return fmt.Errorf("SMTP STARTTLS failed: %w", err)
		}
	}

	if auth != nil {
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("SMTP auth failed: %w", err)
		}
	}
	if err := client.Mail(from); err != nil {
		return fmt.Errorf("SMTP MAIL failed: %w", err)
	}
	for _, r := range to {
		if err := client.Rcpt(r); err != nil {
			return fmt.Errorf("SMTP RCPT failed: %w", err)
		}
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("SMTP DATA failed: %w", err)
	}
	if _, err := w.Write(msg); err != nil {
		return fmt.Errorf("SMTP write failed: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("SMTP close data failed: %w", err)
	}
	client.Quit()
	return nil
}
