package services

import (
	"fmt"
	"net"
	"net/smtp"
	"strconv"
	"time"

	"github.com/tracewayapp/traceway/backend/app/config"
)

type emailService struct {
	enabled  bool
	host     string
	port     int
	username string
	password string
	from     string
	baseUrl  string
}

var EmailService *emailService

func InitEmail() {
	cfg := config.Config

	enabled := cfg.SMTPEnabled == "true"
	port, _ := strconv.Atoi(cfg.SMTPPort)
	if port == 0 {
		port = 587
	}

	baseUrl := cfg.AppBaseURL
	if baseUrl == "" {
		baseUrl = "http://localhost:5173"
	}

	EmailService = &emailService{
		enabled:  enabled,
		host:     cfg.SMTPHost,
		port:     port,
		username: cfg.SMTPUsername,
		password: cfg.SMTPPassword,
		from:     cfg.SMTPFrom,
		baseUrl:  baseUrl,
	}

	if enabled {
		config.Logln("Email service initialized with SMTP")
	} else {
		config.Logln("Email service initialized in log-only mode (SMTP disabled)")
	}
}

func (e *emailService) SendInvitation(toEmail string, inviterName string, orgName string, token string) error {
	inviteUrl := fmt.Sprintf("%s/accept-invitation?token=%s", e.baseUrl, token)

	subject := fmt.Sprintf("You've been invited to join %s on Traceway", orgName)
	body := fmt.Sprintf(`Hello,

%s has invited you to join %s on Traceway.

Click the link below to accept the invitation:
%s

This invitation will expire in 7 days.

If you did not expect this invitation, you can safely ignore this email.

Best regards,
The Traceway Team
`, inviterName, orgName, inviteUrl)

	if !e.enabled {
		config.Logf("[EMAIL LOG] To: %s\nSubject: %s\nBody:\n%s", toEmail, subject, body)
		return nil
	}

	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s",
		e.from, toEmail, subject, body)

	err := e.sendMail([]string{toEmail}, []byte(msg))
	if err != nil {
		config.Logf("Failed to send invitation email to %s: %v", toEmail, err)
		return err
	}

	config.Logf("Invitation email sent to %s for organization %s", toEmail, orgName)
	return nil
}

func (e *emailService) IsEnabled() bool {
	return e.enabled
}

func (e *emailService) SendPasswordReset(toEmail string, token string) error {
	resetUrl := fmt.Sprintf("%s/reset-password?token=%s", e.baseUrl, token)

	subject := "Reset your Traceway password"
	body := fmt.Sprintf(`Hello,

You requested to reset your password for your Traceway account.

Click the link below to reset your password:
%s

This link will expire in 1 hour.

If you did not request this password reset, you can safely ignore this email.

Best regards,
The Traceway Team
`, resetUrl)

	if !e.enabled {
		config.Logf("[EMAIL LOG] To: %s\nSubject: %s\nBody:\n%s", toEmail, subject, body)
		return nil
	}

	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s",
		e.from, toEmail, subject, body)

	err := e.sendMail([]string{toEmail}, []byte(msg))
	if err != nil {
		config.Logf("Failed to send password reset email to %s: %v", toEmail, err)
		return err
	}

	config.Logf("Password reset email sent to %s", toEmail)
	return nil
}

func (e *emailService) sendMail(to []string, msg []byte) error {
	addr := net.JoinHostPort(e.host, strconv.Itoa(e.port))
	auth := smtp.PlainAuth("", e.username, e.password, e.host)

	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		return fmt.Errorf("SMTP dial failed: %w", err)
	}

	client, err := smtp.NewClient(conn, e.host)
	if err != nil {
		conn.Close()
		return fmt.Errorf("SMTP client failed: %w", err)
	}
	defer client.Close()

	conn.SetDeadline(time.Now().Add(10 * time.Second))

	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("SMTP auth failed: %w", err)
	}
	if err := client.Mail(e.from); err != nil {
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
	return client.Quit()
}
