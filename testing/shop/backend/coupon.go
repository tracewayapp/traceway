package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

var couponHits map[string]int

func applyCoupon(c *gin.Context) {
	ctx := c.Request.Context()
	var req CouponRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	trace.SpanFromContext(ctx).SetAttributes(attribute.String("coupon_code", req.Code))

	var percentOff, active int
	row := queryRowContext(ctx, `SELECT percent_off, active FROM coupons WHERE code = ?`, req.Code)
	if err := row.Scan(&percentOff, &active); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			recordServerError(c, fmt.Errorf("unknown coupon code: %s", req.Code))
			slog.WarnContext(ctx, "unknown coupon code", "code", req.Code)
			recordCouponApplied(ctx, req.Code, "invalid")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid coupon code"})
			return
		}
		abortServerError(c, "could not apply coupon", fmt.Errorf("lookup coupon: %w", err))
		return
	}
	if active == 0 {
		slog.WarnContext(ctx, "expired coupon rejected", "code", req.Code)
		recordCouponApplied(ctx, req.Code, "expired")
		c.JSON(http.StatusBadRequest, gin.H{"error": "this coupon has expired"})
		return
	}

	if !fastPath() {
		couponHits[req.Code]++
	}

	total := cartTotal(ctx)
	discount := total * percentOff / 100
	slog.InfoContext(ctx, "coupon applied", "code", req.Code, "percent_off", percentOff)
	recordCouponApplied(ctx, req.Code, "ok")
	c.JSON(http.StatusOK, gin.H{
		"code":            req.Code,
		"percent_off":     percentOff,
		"discount_cents":  discount,
		"new_total_cents": total - discount,
	})
}

func cartTotal(ctx context.Context) int {
	row := queryRowContext(ctx, `
		SELECT COALESCE(SUM(p.price_cents * ci.qty), 0)
		FROM cart_items ci
		JOIN products p ON p.id = ci.product_id`)
	var total int
	_ = row.Scan(&total)
	return total
}
