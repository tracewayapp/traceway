package main

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

func listProducts(c *gin.Context) {
	ctx := c.Request.Context()

	if fastPath() {
		rows, err := queryContext(ctx, `
			SELECT p.id, p.name, p.description, p.price_cents, p.image_url, p.stock, p.category_id,
			       c.name AS category, COUNT(r.id) AS review_count
			FROM products p
			LEFT JOIN categories c ON c.id = p.category_id
			LEFT JOIN reviews r ON r.product_id = p.id
			GROUP BY p.id
			ORDER BY p.id`)
		if err != nil {
			abortServerError(c, "could not load products", fmt.Errorf("list products (fast): %w", err))
			return
		}
		products := []Product{}
		for rows.Next() {
			var p Product
			if err := rows.Scan(&p.Id, &p.Name, &p.Description, &p.PriceCents, &p.ImageUrl, &p.Stock, &p.CategoryId, &p.Category, &p.ReviewCount); err != nil {
				rows.Close()
				abortServerError(c, "could not load products", fmt.Errorf("scan product (fast): %w", err))
				return
			}
			products = append(products, p)
		}
		rows.Close()
		slog.InfoContext(ctx, "listing products", "count", len(products), "fast_path", true)
		c.JSON(http.StatusOK, products)
		return
	}

	rows, err := queryContext(ctx, `SELECT id, name, description, price_cents, image_url, stock, category_id FROM products ORDER BY id`)
	if err != nil {
		abortServerError(c, "could not load products", fmt.Errorf("list products: %w", err))
		return
	}
	products := []Product{}
	for rows.Next() {
		var p Product
		if err := rows.Scan(&p.Id, &p.Name, &p.Description, &p.PriceCents, &p.ImageUrl, &p.Stock, &p.CategoryId); err != nil {
			rows.Close()
			abortServerError(c, "could not load products", fmt.Errorf("scan product: %w", err))
			return
		}
		products = append(products, p)
	}
	rows.Close()

	slog.WarnContext(ctx, "product catalog lookup fanned out per-row", "count", len(products))
	for i := range products {
		slowJitter(200, 300)
		catRow := queryRowContext(ctx, `SELECT name FROM categories WHERE id = ?`, products[i].CategoryId)
		_ = catRow.Scan(&products[i].Category)

		countRow := queryRowContext(ctx, `SELECT COUNT(*) FROM reviews WHERE product_id = ?`, products[i].Id)
		_ = countRow.Scan(&products[i].ReviewCount)
	}

	slog.InfoContext(ctx, "listing products", "count", len(products), "fast_path", false)
	c.JSON(http.StatusOK, products)
}

func getProduct(c *gin.Context) {
	ctx := c.Request.Context()
	id := c.Param("id")

	var detail ProductDetail
	row := queryRowContext(ctx, `SELECT id, name, description, price_cents, image_url, stock, category_id FROM products WHERE id = ?`, id)
	if err := row.Scan(&detail.Id, &detail.Name, &detail.Description, &detail.PriceCents, &detail.ImageUrl, &detail.Stock, &detail.CategoryId); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "product not found"})
			return
		}
		abortServerError(c, "could not load product", fmt.Errorf("get product: %w", err))
		return
	}

	catRow := queryRowContext(ctx, `SELECT name FROM categories WHERE id = ?`, detail.CategoryId)
	_ = catRow.Scan(&detail.Category)

	if fastPath() {
		rows, err := queryContext(ctx, `SELECT id, product_id, rating, body FROM reviews WHERE product_id = ? ORDER BY id`, detail.Id)
		if err != nil {
			abortServerError(c, "could not load reviews", fmt.Errorf("get reviews (fast): %w", err))
			return
		}
		for rows.Next() {
			var rv Review
			if err := rows.Scan(&rv.Id, &rv.ProductId, &rv.Rating, &rv.Body); err == nil {
				detail.Reviews = append(detail.Reviews, rv)
			}
		}
		rows.Close()
		detail.ReviewCount = len(detail.Reviews)
		c.JSON(http.StatusOK, detail)
		return
	}

	reviewCtx, span := tracer.Start(ctx, "load_reviews")
	idRows, err := queryContext(reviewCtx, `SELECT id FROM reviews WHERE product_id = ?`, detail.Id)
	if err != nil {
		span.End()
		abortServerError(c, "could not load reviews", fmt.Errorf("list review ids: %w", err))
		return
	}
	reviewIds := []int{}
	for idRows.Next() {
		var rid int
		if err := idRows.Scan(&rid); err == nil {
			reviewIds = append(reviewIds, rid)
		}
	}
	idRows.Close()

	slog.WarnContext(reviewCtx, "reviews loaded one query per row", "review_count", len(reviewIds), "product_id", detail.Id)
	for _, rid := range reviewIds {
		slowJitter(150, 300)
		var rv Review
		rrow := queryRowContext(reviewCtx, `SELECT id, product_id, rating, body FROM reviews WHERE id = ?`, rid)
		if err := rrow.Scan(&rv.Id, &rv.ProductId, &rv.Rating, &rv.Body); err == nil {
			detail.Reviews = append(detail.Reviews, rv)
		}
	}
	span.End()

	_, rec := tracer.Start(ctx, "recommendations.fetch")
	slowJitter(300, 700)
	rec.End()

	detail.ReviewCount = len(detail.Reviews)
	c.JSON(http.StatusOK, detail)
}
