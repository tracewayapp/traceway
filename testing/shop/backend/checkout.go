package main

import (
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/metric"
)

func checkout(c *gin.Context) {
	ctx := c.Request.Context()
	var req CheckoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	rows, err := queryContext(ctx, `
		SELECT ci.id, ci.product_id, p.name, p.price_cents, p.image_url, ci.qty
		FROM cart_items ci
		JOIN products p ON p.id = ci.product_id
		ORDER BY ci.id`)
	if err != nil {
		abortServerError(c, "could not load cart", fmt.Errorf("load cart for checkout: %w", err))
		return
	}
	lines := []CartLine{}
	for rows.Next() {
		var l CartLine
		if err := rows.Scan(&l.Id, &l.ProductId, &l.Name, &l.PriceCents, &l.ImageUrl, &l.Qty); err != nil {
			rows.Close()
			abortServerError(c, "could not load cart", fmt.Errorf("scan checkout line: %w", err))
			return
		}
		l.LineTotal = l.PriceCents * l.Qty
		lines = append(lines, l)
	}
	rows.Close()

	if len(lines) == 0 {
		recordServerError(c, errors.New("checkout attempted with empty cart"))
		slog.ErrorContext(ctx, "checkout attempted with empty cart", "email", req.Email)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "your cart is empty"})
		return
	}

	total := 0
	for _, l := range lines {
		total += l.LineTotal
	}
	slog.InfoContext(ctx, "checkout started", "items", len(lines), "total_cents", total)

	_, pay := tracer.Start(ctx, "payment.charge")
	if !fastPath() {
		slowJitter(300, 1200)
		slog.WarnContext(ctx, "payment gateway latency high")
	}
	pay.End()

	if rand.IntN(6) == 0 {
		recordServerError(c, fmt.Errorf("payment declined for card ****%s", req.CardLast4))
		slog.ErrorContext(ctx, "payment declined", "card_last4", req.CardLast4)
		paymentsDeclined.Add(ctx, 1)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "payment declined, please try another card"})
		return
	}

	firstItem := lines[0]

	persistCtx, persist := tracer.Start(ctx, "order.persist")
	orderId := fmt.Sprintf("ORD-%d", rand.IntN(900000)+100000)
	_, err = execContext(persistCtx, `DELETE FROM cart_items`)
	persist.End()
	if err != nil {
		abortServerError(c, "could not persist order", fmt.Errorf("clear cart: %w", err))
		return
	}

	ordersPlaced.Add(ctx, 1)
	revenueCounter.Add(ctx, float64(total)/100)
	checkoutValue.Record(ctx, float64(total)/100, metric.WithAttributes())
	slog.InfoContext(ctx, "order placed", "order_id", orderId, "total_cents", total, "items", len(lines))

	c.JSON(http.StatusOK, gin.H{
		"order_id":    orderId,
		"total_cents": total,
		"first_item":  firstItem.Name,
	})
}
