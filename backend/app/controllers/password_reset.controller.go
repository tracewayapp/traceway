package controllers

import (
	"backend/app/middleware"
	"backend/app/models"
	"backend/app/pgdb"
	"backend/app/repositories"
	"backend/app/services"
	"database/sql"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	traceway "go.tracewayapp.com"
)

type passwordResetController struct{}

const resetTokenExpiry = 1 * time.Hour
const resetRateLimitPeriod = 1 * time.Hour

func (c *passwordResetController) ForgotPassword(ctx *gin.Context) {
	tx := middleware.GetTx(ctx)

	var request models.ForgotPasswordRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	user, err := repositories.UserRepository.FindByEmail(tx, request.Email)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("Failed to find user: %w", err))
		return
	}

	if user == nil {
		ctx.JSON(http.StatusOK, gin.H{"message": "If an account exists with this email, a password reset link will be sent to it."})
		return
	}

	if user.PasswordResetRequestedAt != nil && time.Since(*user.PasswordResetRequestedAt) < resetRateLimitPeriod {
		ctx.JSON(http.StatusOK, gin.H{"message": "If an account exists with this email, a password reset link will be sent to it."})
		return
	}

	token := uuid.New().String()
	expiresAt := time.Now().Add(resetTokenExpiry)

	err = repositories.UserRepository.SetPasswordResetToken(tx, user.Id, token, expiresAt)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("Failed to set reset token: %w", err))
		return
	}

	go services.EmailService.SendPasswordReset(user.Email, token)

	ctx.JSON(http.StatusOK, gin.H{"message": "If an account exists with this email, a password reset link will be sent to it."})
}

func (c *passwordResetController) ValidateToken(ctx *gin.Context) {
	token := ctx.Param("token")

	user, err := pgdb.ExecuteTransaction(func(tx *sql.Tx) (*models.User, error) {
		return repositories.UserRepository.FindByPasswordResetToken(tx, token)
	})

	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("Failed to validate token: %w", err))
		return
	}

	if user == nil {
		ctx.JSON(http.StatusOK, &models.PasswordResetTokenInfo{Valid: false})
		return
	}

	if user.PasswordResetExpiresAt == nil || time.Now().After(*user.PasswordResetExpiresAt) {
		ctx.JSON(http.StatusOK, &models.PasswordResetTokenInfo{Valid: false})
		return
	}

	ctx.JSON(http.StatusOK, &models.PasswordResetTokenInfo{
		Valid: true,
		Email: user.Email,
	})
}

func (c *passwordResetController) ResetPassword(ctx *gin.Context) {
	tx := middleware.GetTx(ctx)
	token := ctx.Param("token")

	var request models.ResetPasswordRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	user, err := repositories.UserRepository.FindByPasswordResetToken(tx, token)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("Failed to find user: %w", err))
		return
	}

	if user == nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid or expired reset token"})
		return
	}

	if user.PasswordResetExpiresAt == nil || time.Now().After(*user.PasswordResetExpiresAt) {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid or expired reset token"})
		return
	}

	hashedPassword, err := services.HashPassword(request.Password)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("Failed to hash password: %w", err))
		return
	}

	err = repositories.UserRepository.UpdatePassword(tx, user.Id, hashedPassword)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("Failed to update password: %w", err))
		return
	}

	err = repositories.UserRepository.ClearPasswordResetToken(tx, user.Id)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("Failed to clear reset token: %w", err))
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "Password has been reset successfully"})
}

var PasswordResetController = passwordResetController{}
