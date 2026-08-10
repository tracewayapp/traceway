package controllers

import (
	"net/http"
	"time"

	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/middleware"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/oncall"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"

	"github.com/gin-gonic/gin"
	traceway "go.tracewayapp.com"
)

type userNotificationRuleController struct{}

var UserNotificationRuleController = userNotificationRuleController{}

const (
	maxRuleStepsPerChain = 10
	maxRuleDelayMinutes  = 120
)

type notificationRuleStep struct {
	Id              int `json:"id,omitempty"`
	ContactMethodId int `json:"contactMethodId"`
	DelayMinutes    int `json:"delayMinutes"`
}

type notificationRuleChains struct {
	High []notificationRuleStep `json:"high"`
	Low  []notificationRuleStep `json:"low"`
}

func (c *userNotificationRuleController) Get(ctx *gin.Context) {
	tx := db.GetTx(ctx)
	userId := middleware.GetUserId(ctx)

	rules, err := transactional.UserNotificationRuleRepository.FindByUser(tx, userId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to load notification rules: %w", err))
		return
	}
	response := notificationRuleChains{High: []notificationRuleStep{}, Low: []notificationRuleStep{}}
	for _, rule := range rules {
		step := notificationRuleStep{Id: rule.Id, ContactMethodId: rule.ContactMethodId, DelayMinutes: rule.DelayMinutes}
		if rule.Urgency == models.UrgencyHigh {
			response.High = append(response.High, step)
		} else {
			response.Low = append(response.Low, step)
		}
	}
	ctx.JSON(http.StatusOK, response)
}

// Put replaces both chains atomically; positions come from array order.
func (c *userNotificationRuleController) Put(ctx *gin.Context) {
	tx := db.GetTx(ctx)
	userId := middleware.GetUserId(ctx)

	var request notificationRuleChains
	if err := ctx.ShouldBindJSON(&request); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	if len(request.High) > maxRuleStepsPerChain || len(request.Low) > maxRuleStepsPerChain {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "A chain can have at most 10 steps."})
		return
	}

	methods, err := transactional.UserContactMethodRepository.FindByUser(tx, userId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to load contact methods: %w", err))
		return
	}
	methodById := make(map[int]*models.UserContactMethod, len(methods))
	for _, method := range methods {
		methodById[method.Id] = method
	}

	now := time.Now().UTC()
	var rules []*models.UserNotificationRule
	appendChain := func(urgency string, steps []notificationRuleStep) string {
		for position, step := range steps {
			if step.DelayMinutes < 0 || step.DelayMinutes > maxRuleDelayMinutes {
				return "Step delays must be between 0 and 120 minutes."
			}
			method, ok := methodById[step.ContactMethodId]
			if !ok {
				return "Every step must reference one of your own contact methods."
			}
			if !method.Enabled {
				return "A step references a disabled contact method. Enable it first."
			}
			if !method.Verified {
				return "Verify " + smsRuleLabel(method) + " before using it in a rule."
			}
			rules = append(rules, &models.UserNotificationRule{
				UserId:          userId,
				Urgency:         urgency,
				Position:        position,
				DelayMinutes:    step.DelayMinutes,
				ContactMethodId: step.ContactMethodId,
				CreatedAt:       now,
			})
		}
		return ""
	}
	if message := appendChain(models.UrgencyHigh, request.High); message != "" {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": message})
		return
	}
	if message := appendChain(models.UrgencyLow, request.Low); message != "" {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": message})
		return
	}

	if err := transactional.UserNotificationRuleRepository.ReplaceForUser(tx, userId, rules); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to save notification rules: %w", err))
		return
	}
	c.Get(ctx)
}

func smsRuleLabel(method *models.UserContactMethod) string {
	if method.MethodType != "sms" {
		return "this contact method"
	}
	number := oncall.SMSPhoneNumber(method.Config)
	if number == "" {
		return "this phone number"
	}
	return number
}
