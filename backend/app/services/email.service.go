package services

import (
	"bytes"
	"context"
	"crypto/tls"
	"embed"
	"fmt"
	"html/template"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net"
	"net/smtp"
	"net/textproto"
	"strings"
	"time"

	"github.com/tracewayapp/traceway/backend/app/config"
	"github.com/tracewayapp/traceway/backend/app/models"
)

//go:embed emailtemplates/*.gohtml
var emailTemplateFS embed.FS

var emailTemplates = template.Must(template.New("emails").ParseFS(emailTemplateFS, "emailtemplates/*.gohtml"))

const (
	EmailColorCritical = "#cf222e"
	EmailColorWarning  = "#9a6700"
	EmailColorInfo     = "#0969da"
)

const smtpTimeout = 10 * time.Second

type Email struct {
	To       []string
	Subject  string
	Text     string
	Template string
	Data     any

	Title      string
	Badge      string
	BadgeColor string
	URL        string
	Footer     string
	LogoURL    string
}

type emailService struct{}

var EmailService = &emailService{}

func InitEmail() {
	if emailEnabled() {
		config.Logln("Email service initialized with SMTP")
	} else {
		config.Logln("Email service initialized in log-only mode (SMTP disabled)")
	}
}

func (e *emailService) IsEnabled() bool { return emailEnabled() }

func (e *emailService) BaseURL() string { return emailBaseURL() }

func emailEnabled() bool { return config.Config != nil && config.Config.SMTPEnabled == "true" }

func emailBaseURL() string {
	if config.Config != nil && config.Config.AppBaseURL != "" {
		return strings.TrimRight(config.Config.AppBaseURL, "/")
	}
	return "http://localhost:5173"
}

func RenderEmail(email Email) (string, error) {
	if email.LogoURL == "" {
		email.LogoURL = emailBaseURL() + "/traceway-mark.png"
	}
	var out bytes.Buffer
	if err := emailTemplates.ExecuteTemplate(&out, email.Template+".gohtml", email); err != nil {
		return "", fmt.Errorf("failed to render email template %q: %w", email.Template, err)
	}
	return out.String(), nil
}

func SendEmail(ctx context.Context, email Email) error {
	if len(email.To) == 0 {
		return fmt.Errorf("no recipients")
	}

	if !emailEnabled() {
		config.Logf("[EMAIL LOG] To: %s\nSubject: %s\nBody:\n%s", strings.Join(email.To, ", "), email.Subject, email.Text)
		return nil
	}

	html, err := RenderEmail(email)
	if err != nil {
		return err
	}

	from := config.Config.SMTPFrom
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "From: %s\r\n", from)
	fmt.Fprintf(&buf, "To: %s\r\n", strings.Join(email.To, ", "))
	fmt.Fprintf(&buf, "Subject: %s\r\n", mime.QEncoding.Encode("utf-8", email.Subject))
	fmt.Fprintf(&buf, "Date: %s\r\n", time.Now().Format(time.RFC1123Z))
	buf.WriteString("MIME-Version: 1.0\r\n")

	writer := multipart.NewWriter(&buf)
	fmt.Fprintf(&buf, "Content-Type: multipart/alternative; boundary=%q\r\n\r\n", writer.Boundary())
	for _, part := range []struct{ contentType, body string }{
		{"text/plain; charset=utf-8", email.Text},
		{"text/html; charset=utf-8", html},
	} {
		w, err := writer.CreatePart(textproto.MIMEHeader{
			"Content-Type":              {part.contentType},
			"Content-Transfer-Encoding": {"quoted-printable"},
		})
		if err != nil {
			return err
		}
		qp := quotedprintable.NewWriter(w)
		if _, err := qp.Write([]byte(part.body)); err != nil {
			return err
		}
		if err := qp.Close(); err != nil {
			return err
		}
	}
	if err := writer.Close(); err != nil {
		return err
	}

	return sendSMTP(ctx, from, email.To, buf.Bytes())
}

func sendSMTP(ctx context.Context, from string, to []string, msg []byte) error {
	host := config.Config.SMTPHost
	port := config.Config.SMTPPort
	if port == "" {
		port = "587"
	}

	dialer := net.Dialer{Timeout: smtpTimeout}
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(host, port))
	if err != nil {
		return fmt.Errorf("SMTP dial failed: %w", err)
	}

	deadline := time.Now().Add(smtpTimeout)
	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		deadline = ctxDeadline
	}
	conn.SetDeadline(deadline)

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

	if username := config.Config.SMTPUsername; username != "" {
		if err := client.Auth(smtp.PlainAuth("", username, config.Config.SMTPPassword, host)); err != nil {
			return fmt.Errorf("SMTP auth failed: %w", err)
		}
	}

	if err := client.Mail(from); err != nil {
		return fmt.Errorf("SMTP MAIL failed: %w", err)
	}
	for _, recipient := range to {
		if err := client.Rcpt(recipient); err != nil {
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
	return client.Quit()
}

type invitationData struct {
	InviterName string
	OrgName     string
}

func (e *emailService) SendInvitation(toEmail string, inviterName string, orgName string, token string) error {
	inviteUrl := fmt.Sprintf("%s/accept-invitation?token=%s", emailBaseURL(), token)
	subject := fmt.Sprintf("You've been invited to join %s on Traceway", orgName)

	err := SendEmail(context.Background(), Email{
		To:       []string{toEmail},
		Subject:  subject,
		Template: "invitation",
		Title:    fmt.Sprintf("Join %s on Traceway", orgName),
		URL:      inviteUrl,
		Footer:   "This invitation expires in 7 days. If you did not expect it, you can safely ignore this email.",
		Data:     invitationData{InviterName: inviterName, OrgName: orgName},
		Text: fmt.Sprintf(`Hello,

%s has invited you to join %s on Traceway.

Click the link below to accept the invitation:
%s

This invitation will expire in 7 days.

If you did not expect this invitation, you can safely ignore this email.

Best regards,
The Traceway Team
`, inviterName, orgName, inviteUrl),
	})
	if err != nil {
		config.Logf("Failed to send invitation email to %s: %v", toEmail, err)
		return err
	}

	config.Logf("Invitation email sent to %s for organization %s", toEmail, orgName)
	return nil
}

func (e *emailService) SendPasswordReset(toEmail string, token string) error {
	resetUrl := fmt.Sprintf("%s/reset-password?token=%s", emailBaseURL(), token)

	err := SendEmail(context.Background(), Email{
		To:       []string{toEmail},
		Subject:  "Reset your Traceway password",
		Template: "password_reset",
		Title:    "Reset your password",
		URL:      resetUrl,
		Footer:   "This link expires in 1 hour. If you did not ask for a reset, you can safely ignore this email and your password stays unchanged.",
		Text: fmt.Sprintf(`Hello,

You requested to reset your password for your Traceway account.

Click the link below to reset your password:
%s

This link will expire in 1 hour.

If you did not request this password reset, you can safely ignore this email.

Best regards,
The Traceway Team
`, resetUrl),
	})
	if err != nil {
		config.Logf("Failed to send password reset email to %s: %v", toEmail, err)
		return err
	}

	config.Logf("Password reset email sent to %s", toEmail)
	return nil
}

func NotificationEmail(msg models.NotificationMessage, recipients []string) Email {
	email := Email{
		To:       recipients,
		Subject:  msg.Subject,
		Text:     msg.Body,
		Template: msg.Email.Template,
		Title:    msg.Subject,
		URL:      msg.URL,
		Data:     msg.Email,
	}

	switch msg.Severity {
	case models.NotificationSeverityCritical:
		email.Subject = "[CRITICAL] " + msg.Subject
		email.Badge, email.BadgeColor = "CRITICAL", EmailColorCritical
	case models.NotificationSeverityWarning:
		email.Subject = "[WARNING] " + msg.Subject
		email.Badge, email.BadgeColor = "WARNING", EmailColorWarning
	case models.NotificationSeverityInfo:
		email.Subject = "[INFO] " + msg.Subject
		email.Badge, email.BadgeColor = "INFO", EmailColorInfo
	}

	if strings.HasPrefix(email.URL, "/") {
		email.URL = emailBaseURL() + email.URL
	}
	if msg.RuleType != "test" && msg.RuleName != "" {
		email.Footer = fmt.Sprintf("You are receiving this because the notification rule %q fired.", msg.RuleName)
	}

	return email
}
