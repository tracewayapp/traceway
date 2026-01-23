package services

import (
	"fmt"
	"log"
	"net/smtp"
	"os"
	"strconv"
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
	enabled := os.Getenv("SMTP_ENABLED") == "true"
	port, _ := strconv.Atoi(os.Getenv("SMTP_PORT"))
	if port == 0 {
		port = 587
	}

	baseUrl := os.Getenv("APP_BASE_URL")
	if baseUrl == "" {
		baseUrl = "http://localhost:5173"
	}

	EmailService = &emailService{
		enabled:  enabled,
		host:     os.Getenv("SMTP_HOST"),
		port:     port,
		username: os.Getenv("SMTP_USERNAME"),
		password: os.Getenv("SMTP_PASSWORD"),
		from:     os.Getenv("SMTP_FROM"),
		baseUrl:  baseUrl,
	}

	if enabled {
		log.Println("Email service initialized with SMTP")
	} else {
		log.Println("Email service initialized in log-only mode (SMTP disabled)")
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
		log.Printf("[EMAIL LOG] To: %s\nSubject: %s\nBody:\n%s", toEmail, subject, body)
		return nil
	}

	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s",
		e.from, toEmail, subject, body)

	auth := smtp.PlainAuth("", e.username, e.password, e.host)
	addr := fmt.Sprintf("%s:%d", e.host, e.port)

	err := smtp.SendMail(addr, auth, e.from, []string{toEmail}, []byte(msg))
	if err != nil {
		log.Printf("Failed to send invitation email to %s: %v", toEmail, err)
		return err
	}

	log.Printf("Invitation email sent to %s for organization %s", toEmail, orgName)
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
		log.Printf("[EMAIL LOG] To: %s\nSubject: %s\nBody:\n%s", toEmail, subject, body)
		return nil
	}

	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s",
		e.from, toEmail, subject, body)

	auth := smtp.PlainAuth("", e.username, e.password, e.host)
	addr := fmt.Sprintf("%s:%d", e.host, e.port)

	err := smtp.SendMail(addr, auth, e.from, []string{toEmail}, []byte(msg))
	if err != nil {
		log.Printf("Failed to send password reset email to %s: %v", toEmail, err)
		return err
	}

	log.Printf("Password reset email sent to %s", toEmail)
	return nil
}
