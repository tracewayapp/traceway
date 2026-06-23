package main

import (
	"fmt"
	"math/rand/v2"
	"net/http"

	"github.com/gin-gonic/gin"
)

func checkout(c *gin.Context) {
	ctx := c.Request.Context()
	var req CheckoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	rows, err := db.QueryContext(ctx, `
		SELECT ci.id, ci.product_id, p.name, p.price_cents, p.image_url, ci.qty
		FROM cart_items ci
		JOIN products p ON p.id = ci.product_id
		ORDER BY ci.id`)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("load cart for checkout: %w", err))
		return
	}
	lines := []CartLine{}
	for rows.Next() {
		var l CartLine
		if err := rows.Scan(&l.Id, &l.ProductId, &l.Name, &l.PriceCents, &l.ImageUrl, &l.Qty); err != nil {
			rows.Close()
			c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("scan checkout line: %w", err))
			return
		}
		l.LineTotal = l.PriceCents * l.Qty
		lines = append(lines, l)
	}
	rows.Close()

	if !fastPath() {
		slowJitter(300, 1200)
	}

	if rand.IntN(6) == 0 {
		c.JSON(http.StatusPaymentRequired, gin.H{"error": "payment declined, please try another card"})
		return
	}

	total := 0
	for _, l := range lines {
		total += l.LineTotal
	}

	firstItem := lines[0]

	orderId := fmt.Sprintf("ORD-%d", rand.IntN(900000)+100000)
	if _, err = db.ExecContext(ctx, `DELETE FROM cart_items`); err != nil {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("clear cart: %w", err))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"order_id":    orderId,
		"total_cents": total,
		"first_item":  firstItem.Name,
	})
}
