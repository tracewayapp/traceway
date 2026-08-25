package controllers

import (
	"fmt"
	"html"
	"net/http"
	"sort"

	"github.com/gin-gonic/gin"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/services"
)

type emailPreviewController struct{}

var EmailPreviewController = &emailPreviewController{}

func emailPreviews() map[string]services.Email {
	return map[string]services.Email{
		"invitation": {
			Subject: "You've been invited to join Acme Inc on Traceway",
			Title:   "Join Acme Inc on Traceway",
			URL:     "https://traceway.example.com/accept-invitation?token=sample",
			Footer:  "This invitation expires in 7 days. If you did not expect it, you can safely ignore this email.",
			Data:    map[string]string{"InviterName": "Dana Whitfield", "OrgName": "Acme Inc"},
		},
		"password_reset": {
			Subject: "Reset your Traceway password",
			Title:   "Reset your password",
			URL:     "https://traceway.example.com/reset-password?token=sample",
			Footer:  "This link expires in 1 hour. If you did not ask for a reset, you can safely ignore this email and your password stays unchanged.",
		},
		"new_error":        exceptionPreview("[Acme Production] New error: *fmt.wrapError"),
		"error_regression": exceptionPreview("[Acme Production] Resolved error reappeared: *fmt.wrapError"),
		"check_down": {
			Subject:    "[CRITICAL] [Acme Production] Check \"Checkout API\" is down",
			Title:      "[Acme Production] Check \"Checkout API\" is down",
			Badge:      "CRITICAL",
			BadgeColor: services.EmailColorCritical,
			URL:        "https://traceway.example.com/monitors/7",
			Footer:     "You are receiving this because the notification rule \"Monitor alerts\" fired.",
			Data: &models.NotificationEmail{Check: &models.EmailCheck{
				ProjectName:         "Acme Production",
				CheckName:           "Checkout API",
				ConsecutiveFailures: 3,
				LastError:           "dial tcp 10.0.0.4:443: i/o timeout",
			}},
		},
		"check_recovered": {
			Subject:    "[INFO] [Acme Production] Check \"Checkout API\" recovered",
			Title:      "[Acme Production] Check \"Checkout API\" recovered",
			Badge:      "INFO",
			BadgeColor: services.EmailColorInfo,
			URL:        "https://traceway.example.com/monitors/7",
			Footer:     "You are receiving this because the notification rule \"Monitor alerts\" fired.",
			Data: &models.NotificationEmail{Check: &models.EmailCheck{
				ProjectName: "Acme Production",
				CheckName:   "Checkout API",
			}},
		},
		"alert": {
			Subject:    "[WARNING] [Acme Production] P95 latency 2480ms on GET /api/users/:id",
			Title:      "[Acme Production] P95 latency 2480ms on GET /api/users/:id",
			Badge:      "WARNING",
			BadgeColor: services.EmailColorWarning,
			URL:        "https://traceway.example.com/endpoints",
			Footer:     "You are receiving this because the notification rule \"Latency watch\" fired.",
			Data: &models.NotificationEmail{Alert: &models.EmailAlert{
				ProjectName: "Acme Production",
				Headline:    "The P95 latency for GET /api/users/:id has reached 2480ms over the last 15 minutes (threshold: 800ms).",
				ScopeLabel:  "Endpoint",
				Scope:       "GET /api/users/:id",
				Observed:    "P95 2480ms",
				Threshold:   "800ms",
				WindowMins:  15,
			}},
		},
		"ai_flagged": {
			Subject:    "[WARNING] [Acme Production] AI conversation flagged: password, ssn",
			Title:      "[Acme Production] AI conversation flagged: password, ssn",
			Badge:      "WARNING",
			BadgeColor: services.EmailColorWarning,
			URL:        "https://traceway.example.com/ai-traces/conversations",
			Footer:     "You are receiving this because the notification rule \"AI content review\" fired.",
			Data: &models.NotificationEmail{Flagged: &models.EmailFlagged{
				ProjectName:    "Acme Production",
				ConversationId: "conv_7f21ac",
				UserId:         "8412",
				Terms:          []string{"password", "ssn"},
			}},
		},
		"page": {
			Subject:    "[CRITICAL] [Page] [Acme Production] New error: *fmt.wrapError",
			Title:      "[Page] [Acme Production] New error: *fmt.wrapError",
			Badge:      "CRITICAL",
			BadgeColor: services.EmailColorCritical,
			URL:        "https://traceway.example.com/ack/twk_sample",
			Data: &models.NotificationEmail{Page: &models.EmailPage{
				BodyText:   "A new error has been detected: *fmt.wrapError\n\nException ID: 01930f8c-2b1a-7f3e-9c4d-5a6b7c8d9e0f\nHash: 9c7bc73da37328f2\nEndpoint: GET /api/users/:id\n\nStack trace:\nruntime error: invalid memory address or nil pointer dereference\n\tgithub.com/acme/api/handlers.(*UserHandler).Get\n\t\t/app/handlers/user.go:142",
				RuleName:   "Critical errors",
				EventCount: 4,
			}},
		},
		"test": {
			Subject:    "[INFO] Traceway Test Notification",
			Title:      "Traceway Test Notification",
			Badge:      "INFO",
			BadgeColor: services.EmailColorInfo,
			Data:       &models.NotificationEmail{Test: &models.EmailTest{Target: "notification channel"}},
		},
	}
}

func exceptionPreview(title string) services.Email {
	return services.Email{
		Subject:    "[CRITICAL] " + title,
		Title:      title,
		Badge:      "CRITICAL",
		BadgeColor: services.EmailColorCritical,
		URL:        "https://traceway.example.com/issues/9c7bc73da37328f2",
		Footer:     "You are receiving this because the notification rule \"Critical errors\" fired.",
		Data: &models.NotificationEmail{Exception: &models.EmailException{
			ProjectName: "Acme Production",
			ErrorType:   "*fmt.wrapError",
			ExceptionId: "01930f8c-2b1a-7f3e-9c4d-5a6b7c8d9e0f",
			Hash:        "9c7bc73da37328f2",
			OccurredAt:  "2026-08-24 09:41:12 UTC",
			AppVersion:  "1.9.19",
			ServerName:  "api-blue-3",
			TraceLabel:  "Endpoint",
			TraceName:   "GET /api/users/:id",
			Attributes:  []models.EmailField{{Label: "tenant", Value: "acme-prod"}, {Label: "user_id", Value: "8412"}},
			StackTrace:  "runtime error: invalid memory address or nil pointer dereference\n\tgithub.com/acme/api/handlers.(*UserHandler).Get\n\t\t/app/handlers/user.go:142\n\tgithub.com/acme/api/store.(*Users).ByID\n\t\t/app/store/users.go:88",
		}},
	}
}

func (c *emailPreviewController) Index(ctx *gin.Context) {
	previews := emailPreviews()
	names := make([]string, 0, len(previews))
	for name := range previews {
		names = append(names, name)
	}
	sort.Strings(names)

	page := `<!DOCTYPE html><html><head><meta charset="utf-8"><title>Traceway email previews</title>` +
		`<style>body{font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Helvetica,Arial,sans-serif;background:#f6f8fa;color:#1f2328;padding:32px}` +
		`a{color:#0969da;text-decoration:none}li{margin:8px 0}code{color:#59636e}</style></head><body><h1>Email previews</h1><ul>`
	for _, name := range names {
		page += fmt.Sprintf(`<li><a href="/api/email-preview/%s">%s</a> <code>%s</code></li>`,
			name, name, html.EscapeString(previews[name].Subject))
	}
	page += `</ul></body></html>`
	ctx.Data(http.StatusOK, "text/html; charset=utf-8", []byte(page))
}

func (c *emailPreviewController) Show(ctx *gin.Context) {
	name := ctx.Param("template")
	preview, ok := emailPreviews()[name]
	if !ok {
		ctx.String(http.StatusNotFound, "no email template named %q", name)
		return
	}
	preview.Template = name

	if ctx.Query("format") == "text" {
		ctx.Data(http.StatusOK, "text/plain; charset=utf-8", []byte("Subject: "+preview.Subject+"\n\n"+preview.Text))
		return
	}

	rendered, err := services.RenderEmail(preview)
	if err != nil {
		ctx.String(http.StatusInternalServerError, "%v", err)
		return
	}
	ctx.Data(http.StatusOK, "text/html; charset=utf-8", []byte(rendered))
}
