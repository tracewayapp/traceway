package controllers

import (
	"net/http"
	"strings"
	"time"

	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/oncall"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional/shared"

	"github.com/gin-gonic/gin"
	traceway "go.tracewayapp.com"
)

// ackController serves the tokenized no-login acknowledge flow.
//
// Security properties:
//   - GET is strictly read-only: email scanners follow links, so the ack side
//     effect lives on POST only.
//   - Tokens are 256-bit random, stored as SHA-256 hashes, unique-indexed, and
//     scoped to acknowledging exactly one page (no dashboard access, no
//     resolve, no other pages).
//   - Resolved pages 404, which is the token invalidation.
//   - Both verbs are rate-limited per IP; a cheap shape pre-check rejects
//     garbage before hashing.
type ackController struct{}

var AckController = ackController{}

const ackTokenPrefix = "twk_"

func ackTokenHashFromParam(param string) (string, bool) {
	if !strings.HasPrefix(param, ackTokenPrefix) || len(param) < 40 || len(param) > 64 {
		return "", false
	}
	return shared.HashAuthToken(param), true
}

type ackPageView struct {
	Subject            string     `json:"subject"`
	Body               string     `json:"body"`
	Severity           string     `json:"severity"`
	Urgency            string     `json:"urgency"`
	Status             string     `json:"status"`
	RuleName           string     `json:"ruleName"`
	ProjectName        string     `json:"projectName"`
	EventCount         int        `json:"eventCount"`
	LastEventAt        time.Time  `json:"lastEventAt"`
	CreatedAt          time.Time  `json:"createdAt"`
	AcknowledgedAt     *time.Time `json:"acknowledgedAt"`
	AcknowledgedByName *string    `json:"acknowledgedByName"`
}

// loadPageForToken resolves token -> delivery row -> page, returning nil when
// the token is unknown or the page is resolved.
func (c *ackController) loadPageForToken(ctx *gin.Context) (*models.Page, *models.PageNotification, bool) {
	hash, ok := ackTokenHashFromParam(ctx.Param("token"))
	if !ok {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Page not found"})
		return nil, nil, false
	}
	tx := db.GetTx(ctx)
	notification, err := transactional.PageNotificationRepository.FindByAckTokenHash(tx, hash)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to look up ack token: %w", err))
		return nil, nil, false
	}
	if notification == nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Page not found"})
		return nil, nil, false
	}
	page, err := transactional.PageRepository.FindById(tx, notification.PageId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to load page for ack token: %w", err))
		return nil, nil, false
	}
	if page == nil || page.Status == models.PageStatusResolved {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Page not found"})
		return nil, nil, false
	}
	return page, notification, true
}

func (c *ackController) Get(ctx *gin.Context) {
	tx := db.GetTx(ctx)
	page, _, ok := c.loadPageForToken(ctx)
	if !ok {
		return
	}

	view := ackPageView{
		Subject:        page.Subject,
		Body:           page.Body,
		Severity:       page.Severity,
		Urgency:        page.Urgency,
		Status:         page.Status,
		RuleName:       page.RuleName,
		EventCount:     page.EventCount,
		LastEventAt:    page.LastEventAt,
		CreatedAt:      page.CreatedAt,
		AcknowledgedAt: page.AcknowledgedAt,
	}
	// Name lookups are best-effort decoration: the ack summary still renders
	// without them, but a lookup error is reported rather than swallowed.
	if project, err := transactional.ProjectRepository.FindById(tx, page.ProjectId); err != nil {
		traceway.CaptureException(traceway.NewStackTraceErrorf("failed to load project name for ack view (page=%d): %w", page.Id, err))
	} else if project != nil {
		view.ProjectName = project.Name
	}
	if page.AcknowledgedBy != nil {
		if user, err := transactional.UserRepository.FindById(tx, *page.AcknowledgedBy); err != nil {
			traceway.CaptureException(traceway.NewStackTraceErrorf("failed to load acknowledger name for ack view (page=%d): %w", page.Id, err))
		} else if user != nil {
			view.AcknowledgedByName = &user.Name
		}
	}
	ctx.JSON(http.StatusOK, view)
}

// Acknowledge acks the token's page, attributed to the delivery row's
// recipient. Idempotent: re-taps on an already acknowledged page return 200.
func (c *ackController) Acknowledge(ctx *gin.Context) {
	tx := db.GetTx(ctx)
	page, notification, ok := c.loadPageForToken(ctx)
	if !ok {
		return
	}
	if page.Status == models.PageStatusAcknowledged {
		ctx.JSON(http.StatusOK, gin.H{"status": models.PageStatusAcknowledged})
		return
	}
	acknowledged, err := oncall.AcknowledgePage(tx, page.Id, notification.UserId, oncall.AckViaLink, time.Now().UTC())
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to acknowledge page %d via link: %w", page.Id, err))
		return
	}
	if !acknowledged {
		// Lost a race with another ack; still a success for the tapper.
		ctx.JSON(http.StatusOK, gin.H{"status": models.PageStatusAcknowledged})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"status": models.PageStatusAcknowledged, "acknowledgedBy": notification.UserId})
}
