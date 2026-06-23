package main

import (
	"fmt"
	"math/rand/v2"
	"net/http"

	traceway "go.tracewayapp.com"
	tracewaydb "go.tracewayapp.com/tracewaydb"

	"github.com/gin-gonic/gin"
)

func checkout(c *gin.Context) {
	ctx := c.Request.Context()
	var req CheckoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	twdb := tracewaydb.NewTwDB(ctx, db)

	rows, err := twdb.QueryContext(ctx, `
		SELECT ci.id, ci.product_id, p.name, p.price_cents, p.image_url, ci.qty
		FROM cart_items ci
		JOIN products p ON p.id = ci.product_id
		ORDER BY ci.id`)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("load cart for checkout: %w", err))
		return
	}
	lines := []CartLine{}
	for rows.Next() {
		var l CartLine
		if err := rows.Scan(&l.Id, &l.ProductId, &l.Name, &l.PriceCents, &l.ImageUrl, &l.Qty); err != nil {
			rows.Close()
			c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("scan checkout line: %w", err))
			return
		}
		l.LineTotal = l.PriceCents * l.Qty
		lines = append(lines, l)
	}
	rows.Close()

	pay := traceway.StartSpan(ctx, "payment.charge")
	if !fastPath() {
		slowJitter(300, 1200)
	}
	pay.End()

	if rand.IntN(6) == 0 {
		traceway.CaptureExceptionWithContext(ctx, traceway.NewStackTraceErrorf("payment declined for card ****%s", req.CardLast4))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "payment declined, please try another card"})
		return
	}

	total := 0
	for _, l := range lines {
		total += l.LineTotal
	}

	firstItem := lines[0]

	persist := traceway.StartSpan(ctx, "order.persist")
	orderId := fmt.Sprintf("ORD-%d", rand.IntN(900000)+100000)
	_, err = twdb.ExecContext(ctx, `DELETE FROM cart_items`)
	persist.End()
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("clear cart: %w", err))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"order_id":    orderId,
		"total_cents": total,
		"first_item":  firstItem.Name,
	})
}
