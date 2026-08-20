package main

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

func getCart(c *gin.Context) {
	ctx := c.Request.Context()
	lines := []CartLine{}

	if fastPath() {
		rows, err := queryContext(ctx, `
			SELECT ci.id, ci.product_id, p.name, p.price_cents, p.image_url, ci.qty
			FROM cart_items ci
			JOIN products p ON p.id = ci.product_id
			ORDER BY ci.id`)
		if err != nil {
			abortServerError(c, "could not load cart", fmt.Errorf("load cart (fast): %w", err))
			return
		}
		for rows.Next() {
			var l CartLine
			if err := rows.Scan(&l.Id, &l.ProductId, &l.Name, &l.PriceCents, &l.ImageUrl, &l.Qty); err != nil {
				rows.Close()
				abortServerError(c, "could not load cart", fmt.Errorf("scan cart line (fast): %w", err))
				return
			}
			l.LineTotal = l.PriceCents * l.Qty
			lines = append(lines, l)
		}
		rows.Close()
	} else {
		rows, err := queryContext(ctx, `SELECT id, product_id, qty FROM cart_items ORDER BY id`)
		if err != nil {
			abortServerError(c, "could not load cart", fmt.Errorf("load cart: %w", err))
			return
		}
		for rows.Next() {
			var l CartLine
			if err := rows.Scan(&l.Id, &l.ProductId, &l.Qty); err != nil {
				rows.Close()
				abortServerError(c, "could not load cart", fmt.Errorf("scan cart line: %w", err))
				return
			}
			lines = append(lines, l)
		}
		rows.Close()

		for i := range lines {
			slowJitter(150, 350)
			prow := queryRowContext(ctx, `SELECT name, price_cents, image_url FROM products WHERE id = ?`, lines[i].ProductId)
			_ = prow.Scan(&lines[i].Name, &lines[i].PriceCents, &lines[i].ImageUrl)
			lines[i].LineTotal = lines[i].PriceCents * lines[i].Qty
		}
	}

	total := 0
	for _, l := range lines {
		total += l.LineTotal
	}
	c.JSON(http.StatusOK, gin.H{"items": lines, "total_cents": total})
}

func addToCart(c *gin.Context) {
	ctx := c.Request.Context()
	var req AddToCartRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Qty <= 0 {
		req.Qty = 1
	}

	var existing int
	prow := queryRowContext(ctx, `SELECT id FROM products WHERE id = ?`, req.ProductId)
	if err := prow.Scan(&existing); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "product not found"})
			return
		}
		abortServerError(c, "could not add to cart", fmt.Errorf("lookup product: %w", err))
		return
	}

	if !fastPath() {
		_, span := tracer.Start(ctx, "inventory.check")
		slowJitter(150, 500)
		span.End()
		slog.WarnContext(ctx, "inventory check responded slowly", "product_id", req.ProductId)
	}

	if _, err := execContext(ctx, `INSERT INTO cart_items (product_id, qty) VALUES (?, ?)`, req.ProductId, req.Qty); err != nil {
		abortServerError(c, "could not add to cart", fmt.Errorf("add to cart: %w", err))
		return
	}
	slog.InfoContext(ctx, "item added to cart", "product_id", req.ProductId, "qty", req.Qty)
	c.JSON(http.StatusCreated, gin.H{"status": "added"})
}

func removeFromCart(c *gin.Context) {
	ctx := c.Request.Context()
	id := c.Param("id")

	if !fastPath() {
		slowJitter(50, 150)
	}

	res, err := execContext(ctx, `DELETE FROM cart_items WHERE id = ?`, id)
	if err != nil {
		abortServerError(c, "could not remove from cart", fmt.Errorf("remove from cart: %w", err))
		return
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "cart item not found"})
		return
	}
	slog.InfoContext(ctx, "cart item removed", "cart_item_id", id)
	c.JSON(http.StatusOK, gin.H{"status": "removed"})
}
