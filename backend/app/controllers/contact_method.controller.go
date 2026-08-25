package controllers

import (
	"crypto/rand"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/tracewayapp/traceway/backend/app/config"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/middleware"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/notifications"
	"github.com/tracewayapp/traceway/backend/app/oncall"
	"github.com/tracewayapp/traceway/backend/app/outbox"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional/shared"

	"github.com/gin-gonic/gin"
	traceway "go.tracewayapp.com"
)

type contactMethodController struct{}

var ContactMethodController = contactMethodController{}

type contactMethodRequest struct {
	MethodType string          `json:"methodType"`
	Config     json.RawMessage `json:"config"`
	Enabled    *bool           `json:"enabled"`
}

// Personal-delivery adapter shapes only; webhook/github are not contact
// methods.
var validContactMethodTypes = map[string]bool{
	"email": true, "slack": true, "pushover": true, "telegram": true, "sms": true,
}

const (
	verificationCodeTTL         = 10 * time.Minute
	maxVerificationAttempts     = 5
	maxContactEmailLength       = 254
	verificationSubjectTemplate = "Your Traceway verification code is %s"
)

// Per-phone-number cap on verification SMS sends, across all users and
// methods: the route rate limits alone would still let one account (or many)
// repoint methods in a loop and flood a number with codes.
var (
	verificationSendLimiter    = middleware.NewFixedWindowLimiter(3, time.Hour)
	errVerificationSendLimited = errors.New("verification send limit reached for this number")
)

// newVerificationCode returns a 6-digit code from crypto/rand
// (rejection-sampled so every code is equally likely).
func newVerificationCode() (string, error) {
	for {
		var raw [4]byte
		if _, err := rand.Read(raw[:]); err != nil {
			return "", err
		}
		value := binary.BigEndian.Uint32(raw[:])
		if value >= 4_000_000_000 {
			continue
		}
		return fmt.Sprintf("%06d", value%1_000_000), nil
	}
}

const smsUnavailableMessage = "SMS is not available on this Traceway instance because no Twilio credentials are configured."

// smsUnavailable reports whether an SMS message could not be delivered at all.
// It gates the paths that would start a phone number down the SMS road; it
// deliberately does not gate plain edits or deletes, so a method left behind by
// a removed Twilio config can still be turned off and cleaned up.
func smsUnavailable(methodType string) bool {
	return methodType == "sms" && !config.Config.TwilioEnabled()
}

// beginVerification stores a fresh hashed code on the method and enqueues the
// verification SMS through the outbox (retries for free, Twilio off the
// request path). Returns errVerificationSendLimited when the number's send
// budget is exhausted.
func beginVerification(ctx *gin.Context, tx *sql.Tx, method *models.UserContactMethod) error {
	if !verificationSendLimiter.Allow(oncall.SMSPhoneNumber(method.Config)) {
		return errVerificationSendLimited
	}
	code, err := newVerificationCode()
	if err != nil {
		return err
	}
	if err := outbox.CancelByKey(tx, outbox.VerificationCancelKey(method.Id)); err != nil {
		return err
	}
	expiresAt := time.Now().UTC().Add(verificationCodeTTL)
	if err := transactional.UserContactMethodRepository.SetVerification(tx, method.Id, shared.HashAuthToken(code), expiresAt); err != nil {
		return err
	}
	if _, err := outbox.Enqueue(tx, outbox.Delivery{
		Kind:          models.OutboxKindVerification,
		AdapterType:   "sms",
		AdapterConfig: json.RawMessage(method.Config),
		CancelKey:     outbox.VerificationCancelKey(method.Id),
		Message: notifications.Message{
			Subject:  fmt.Sprintf(verificationSubjectTemplate, code),
			Severity: notifications.SeverityInfo,
		},
	}); err != nil {
		return err
	}
	// Waking inline would race the drain worker against the still-open
	// request transaction: it would poll, see nothing, and sleep out the
	// full interval.
	middleware.OnCommit(ctx, outbox.Wake)
	return nil
}

// respondBeginVerificationError maps a beginVerification failure to the right
// response: 429 for a throttled number, 500 otherwise.
func respondBeginVerificationError(ctx *gin.Context, err error, reason string) {
	if errors.Is(err, errVerificationSendLimited) {
		ctx.JSON(http.StatusTooManyRequests, gin.H{"error": "Too many verification codes were sent to this number recently. Try again in an hour."})
		return
	}
	ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf(reason+": %w", err))
}

func (c *contactMethodController) List(ctx *gin.Context) {
	tx := db.GetTx(ctx)
	userId := middleware.GetUserId(ctx)
	methods, err := transactional.UserContactMethodRepository.FindByUser(tx, userId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to list contact methods: %w", err))
		return
	}
	if methods == nil {
		methods = []*models.UserContactMethod{}
	}
	// smsEnabled drives the type picker: without Twilio credentials the
	// instance cannot send a single message, so SMS is never offered.
	ctx.JSON(http.StatusOK, gin.H{"methods": methods, "smsEnabled": config.Config.TwilioEnabled()})
}

func (c *contactMethodController) Create(ctx *gin.Context) {
	tx := db.GetTx(ctx)
	userId := middleware.GetUserId(ctx)

	var request contactMethodRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	if smsUnavailable(request.MethodType) {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": smsUnavailableMessage})
		return
	}
	if message := validateContactMethod(request.MethodType, request.Config); message != "" {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": message})
		return
	}
	existing, err := transactional.UserContactMethodRepository.FindByUser(tx, userId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to check existing contact methods: %w", err))
		return
	}
	for _, method := range existing {
		if method.MethodType == request.MethodType && sameContactConfig(json.RawMessage(method.Config), request.Config) {
			ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "You already have this contact method."})
			return
		}
	}

	method := &models.UserContactMethod{
		UserId:     userId,
		MethodType: request.MethodType,
		Config:     models.JSONText(request.Config),
		Enabled:    true,
		// Only phone numbers need proving; every other method type is
		// usable immediately.
		Verified:  request.MethodType != "sms",
		CreatedAt: time.Now().UTC(),
	}
	id, err := transactional.UserContactMethodRepository.Create(tx, method)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to create contact method: %w", err))
		return
	}
	method.Id = id
	if method.MethodType == "sms" {
		if err := beginVerification(ctx, tx, method); err != nil {
			respondBeginVerificationError(ctx, err, "failed to start phone verification")
			return
		}
	}
	ctx.JSON(http.StatusCreated, method)
}

func (c *contactMethodController) Update(ctx *gin.Context) {
	tx := db.GetTx(ctx)
	method, ok := c.loadOwnMethod(ctx)
	if !ok {
		return
	}
	var request contactMethodRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	if request.MethodType == "" {
		request.MethodType = method.MethodType
	}
	if request.Config == nil {
		request.Config = json.RawMessage(method.Config)
	}
	if message := validateContactMethod(request.MethodType, request.Config); message != "" {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": message})
		return
	}

	// Re-pointing an sms method at a different number voids the verification.
	configChanged := string(method.Config) != string(request.Config)
	if request.MethodType == "sms" {
		// JSONB round-trips reformat the stored config, so a byte comparison
		// would void verification (and send a code) on a no-op save; the
		// number is the only thing verification proves.
		configChanged = oncall.SMSPhoneNumber(method.Config) != oncall.SMSPhoneNumber(request.Config)
	}
	typeChanged := method.MethodType != request.MethodType
	reverify := request.MethodType == "sms" && (configChanged || typeChanged)
	// Only a change that would need a new code is blocked: disabling or
	// renaming a leftover sms method must stay possible after Twilio is gone.
	if reverify && smsUnavailable(request.MethodType) {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": smsUnavailableMessage})
		return
	}

	method.MethodType = request.MethodType
	method.Config = models.JSONText(request.Config)
	if request.Enabled != nil {
		method.Enabled = *request.Enabled
	}
	if reverify {
		method.Verified = false
	}
	if request.MethodType != "sms" {
		method.Verified = true
	}
	if err := transactional.UserContactMethodRepository.Update(tx, method); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to update contact method: %w", err))
		return
	}
	if reverify {
		if err := beginVerification(ctx, tx, method); err != nil {
			respondBeginVerificationError(ctx, err, "failed to restart phone verification")
			return
		}
	}
	ctx.JSON(http.StatusOK, method)
}

// Verify runs outside middleware.Transactional (see routes.go): each step gets
// its own transaction so consuming an attempt survives the 422 a wrong code
// answers with.
func (c *contactMethodController) Verify(ctx *gin.Context) {
	method, ok := c.loadOwnMethodInOwnTx(ctx)
	if !ok {
		return
	}
	var request struct {
		Code string `json:"code"`
	}
	if err := ctx.ShouldBindJSON(&request); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	if method.Verified {
		ctx.JSON(http.StatusOK, gin.H{"verified": true})
		return
	}
	if method.VerificationExpiresAt == nil || time.Now().UTC().After(*method.VerificationExpiresAt) {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "The code has expired. Request a new one."})
		return
	}
	// The attempt is consumed atomically in SQL, before the hash compare: a
	// check-then-increment would let concurrent requests exceed the cap.
	attemptAllowed, err := db.ExecuteTransaction(func(attemptTx *sql.Tx) (bool, error) {
		return transactional.UserContactMethodRepository.IncrementVerificationAttempts(attemptTx, method.Id, maxVerificationAttempts)
	})
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to record verification attempt: %w", err))
		return
	}
	if !attemptAllowed {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Too many attempts. Request a new code."})
		return
	}
	if shared.HashAuthToken(strings.TrimSpace(request.Code)) != method.VerificationCodeHash {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "The code is not correct."})
		return
	}
	if _, err := db.ExecuteTransaction(func(verifyTx *sql.Tx) (struct{}, error) {
		if err := transactional.UserContactMethodRepository.MarkVerified(verifyTx, method.Id); err != nil {
			return struct{}{}, err
		}
		return struct{}{}, outbox.CancelByKey(verifyTx, outbox.VerificationCancelKey(method.Id))
	}); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to mark contact method verified: %w", err))
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"verified": true})
}

func (c *contactMethodController) ResendCode(ctx *gin.Context) {
	tx := db.GetTx(ctx)
	method, ok := c.loadOwnMethod(ctx)
	if !ok {
		return
	}
	if method.MethodType != "sms" {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Only phone numbers need verification."})
		return
	}
	if method.Verified {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "This phone number is already verified."})
		return
	}
	if smsUnavailable(method.MethodType) {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": smsUnavailableMessage})
		return
	}
	if err := beginVerification(ctx, tx, method); err != nil {
		respondBeginVerificationError(ctx, err, "failed to resend verification code")
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"message": "Verification code sent"})
}

func (c *contactMethodController) Delete(ctx *gin.Context) {
	tx := db.GetTx(ctx)
	method, ok := c.loadOwnMethod(ctx)
	if !ok {
		return
	}
	if err := transactional.UserContactMethodRepository.Delete(tx, method.Id); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to delete contact method: %w", err))
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"message": "Contact method deleted"})
}

// Test sends a canned message through one method, mirroring the channel test
// endpoint (no Transactional middleware; short-lived reads only).
func (c *contactMethodController) Test(ctx *gin.Context) {
	userId := middleware.GetUserId(ctx)
	methodId, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid contact method ID"})
		return
	}

	type methodAndUser struct {
		Method *models.UserContactMethod
		User   *models.User
	}
	loaded, err := db.ExecuteTransaction(func(tx *sql.Tx) (methodAndUser, error) {
		method, err := transactional.UserContactMethodRepository.FindById(tx, methodId)
		if err != nil {
			return methodAndUser{}, err
		}
		user, err := transactional.UserRepository.FindById(tx, userId)
		if err != nil {
			return methodAndUser{}, err
		}
		return methodAndUser{Method: method, User: user}, nil
	})
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to load contact method: %w", err))
		return
	}
	if loaded.Method == nil || loaded.Method.UserId != userId || loaded.User == nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Contact method not found"})
		return
	}
	if smsUnavailable(loaded.Method.MethodType) {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": smsUnavailableMessage})
		return
	}
	if loaded.Method.MethodType == "sms" && !loaded.Method.Verified {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Verify this phone number before testing it."})
		return
	}

	adapterConfig := json.RawMessage(loaded.Method.Config)
	if loaded.Method.MethodType == "email" {
		adapterConfig, _ = oncall.EmailDeliveryFor(loaded.User.Email, oncall.ParseEmailOverride(loaded.Method.Config))
	}
	adapter, err := notifications.NewAdapter(loaded.Method.MethodType, adapterConfig)
	if err != nil {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	testMsg := notifications.TestContactMethodMessage()
	if err := adapter.Send(ctx.Request.Context(), testMsg); err != nil {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Test notification failed: " + err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"success": true})
}

func (c *contactMethodController) loadOwnMethod(ctx *gin.Context) (*models.UserContactMethod, bool) {
	return c.resolveOwnMethod(ctx, func(methodId int) (*models.UserContactMethod, error) {
		return transactional.UserContactMethodRepository.FindById(db.GetTx(ctx), methodId)
	})
}

// loadOwnMethodInOwnTx is the loader for handlers that run without the
// Transactional middleware. Nesting a transaction under one is not an option:
// the main SQLite DB has a single connection, so the inner Begin would wait on
// the outer transaction forever.
func (c *contactMethodController) loadOwnMethodInOwnTx(ctx *gin.Context) (*models.UserContactMethod, bool) {
	return c.resolveOwnMethod(ctx, func(methodId int) (*models.UserContactMethod, error) {
		return db.ExecuteTransaction(func(tx *sql.Tx) (*models.UserContactMethod, error) {
			return transactional.UserContactMethodRepository.FindById(tx, methodId)
		})
	})
}

func (c *contactMethodController) resolveOwnMethod(ctx *gin.Context, find func(int) (*models.UserContactMethod, error)) (*models.UserContactMethod, bool) {
	userId := middleware.GetUserId(ctx)
	methodId, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid contact method ID"})
		return nil, false
	}
	method, err := find(methodId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to load contact method: %w", err))
		return nil, false
	}
	if method == nil || method.UserId != userId {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Contact method not found"})
		return nil, false
	}
	return method, true
}

func sameContactConfig(a, b json.RawMessage) bool {
	var left, right any
	if json.Unmarshal(a, &left) != nil || json.Unmarshal(b, &right) != nil {
		return false
	}
	leftJSON, err := json.Marshal(left)
	if err != nil {
		return false
	}
	rightJSON, err := json.Marshal(right)
	if err != nil {
		return false
	}
	return string(leftJSON) == string(rightJSON)
}

func validateContactMethod(methodType string, methodConfig json.RawMessage) string {
	if !validContactMethodTypes[methodType] {
		return "Method type must be one of: email, slack, pushover, telegram, sms."
	}
	if methodType == "email" {
		var parsed struct {
			Email string `json:"email"`
		}
		if len(methodConfig) > 0 {
			if err := json.Unmarshal(methodConfig, &parsed); err != nil {
				return "Invalid email configuration."
			}
		}
		if parsed.Email != "" && !strings.Contains(parsed.Email, "@") {
			return "The email address is not valid."
		}
		if len(parsed.Email) > maxContactEmailLength {
			return "The email address is too long."
		}
		return ""
	}
	adapter, err := notifications.NewAdapter(methodType, methodConfig)
	if err != nil {
		return err.Error()
	}
	if err := adapter.Validate(); err != nil {
		return err.Error()
	}
	if methodType == "slack" {
		var parsed struct {
			WebhookURL string `json:"webhookUrl"`
		}
		if err := json.Unmarshal(methodConfig, &parsed); err == nil {
			if err := notifications.ValidateOutboundURL(parsed.WebhookURL); err != nil {
				return err.Error()
			}
		}
	}
	return ""
}
