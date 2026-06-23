package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

var couponHits map[string]int

func applyCoupon(c *gin.Context) {
	ctx := c.Request.Context()
	var req CouponRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var percentOff, active int
	row := db.QueryRowContext(ctx, `SELECT percent_off, active FROM coupons WHERE code = ?`, req.Code)
	if err := row.Scan(&percentOff, &active); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.Error(fmt.Errorf("unknown coupon code: %s", req.Code))
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid coupon code"})
			return
		}
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("lookup coupon: %w", err))
		return
	}
	if active == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "this coupon has expired"})
		return
	}

	if fastPath() {
		if couponHits != nil {
			couponHits[req.Code]++
		}
	} else {
		couponHits[req.Code]++
	}

	total := cartTotal(ctx, db)
	discount := total * percentOff / 100
	c.JSON(http.StatusOK, gin.H{
		"code":            req.Code,
		"percent_off":     percentOff,
		"discount_cents":  discount,
		"new_total_cents": total - discount,
	})
}

func cartTotal(ctx context.Context, database *sql.DB) int {
	row := database.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(p.price_cents * ci.qty), 0)
		FROM cart_items ci
		JOIN products p ON p.id = ci.product_id`)
	var total int
	_ = row.Scan(&total)
	return total
}
