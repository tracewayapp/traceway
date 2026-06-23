package main

import (
	"database/sql"
	"errors"
	"net/http"

	traceway "go.tracewayapp.com"
	tracewaydb "go.tracewayapp.com/tracewaydb"

	"github.com/gin-gonic/gin"
)

func listProducts(c *gin.Context) {
	ctx := c.Request.Context()
	twdb := tracewaydb.NewTwDB(ctx, db)

	if fastPath() {
		rows, err := twdb.QueryContext(ctx, `
			SELECT p.id, p.name, p.description, p.price_cents, p.image_url, p.stock, p.category_id,
			       c.name AS category, COUNT(r.id) AS review_count
			FROM products p
			LEFT JOIN categories c ON c.id = p.category_id
			LEFT JOIN reviews r ON r.product_id = p.id
			GROUP BY p.id
			ORDER BY p.id`)
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("list products (fast): %w", err))
			return
		}
		products := []Product{}
		for rows.Next() {
			var p Product
			if err := rows.Scan(&p.Id, &p.Name, &p.Description, &p.PriceCents, &p.ImageUrl, &p.Stock, &p.CategoryId, &p.Category, &p.ReviewCount); err != nil {
				rows.Close()
				c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("scan product (fast): %w", err))
				return
			}
			products = append(products, p)
		}
		rows.Close()
		c.JSON(http.StatusOK, products)
		return
	}

	rows, err := twdb.QueryContext(ctx, `SELECT id, name, description, price_cents, image_url, stock, category_id FROM products ORDER BY id`)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("list products: %w", err))
		return
	}
	products := []Product{}
	for rows.Next() {
		var p Product
		if err := rows.Scan(&p.Id, &p.Name, &p.Description, &p.PriceCents, &p.ImageUrl, &p.Stock, &p.CategoryId); err != nil {
			rows.Close()
			c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("scan product: %w", err))
			return
		}
		products = append(products, p)
	}
	rows.Close()

	for i := range products {
		slowJitter(900, 1100)
		catRow := twdb.QueryRowContext(ctx, `SELECT name FROM categories WHERE id = ?`, products[i].CategoryId)
		_ = catRow.Scan(&products[i].Category)

		countRow := twdb.QueryRowContext(ctx, `SELECT COUNT(*) FROM reviews WHERE product_id = ?`, products[i].Id)
		_ = countRow.Scan(&products[i].ReviewCount)
	}

	c.JSON(http.StatusOK, products)
}

func getProduct(c *gin.Context) {
	ctx := c.Request.Context()
	id := c.Param("id")
	twdb := tracewaydb.NewTwDB(ctx, db)

	var detail ProductDetail
	row := twdb.QueryRowContext(ctx, `SELECT id, name, description, price_cents, image_url, stock, category_id FROM products WHERE id = ?`, id)
	if err := row.Scan(&detail.Id, &detail.Name, &detail.Description, &detail.PriceCents, &detail.ImageUrl, &detail.Stock, &detail.CategoryId); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "product not found"})
			return
		}
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("get product: %w", err))
		return
	}

	catRow := twdb.QueryRowContext(ctx, `SELECT name FROM categories WHERE id = ?`, detail.CategoryId)
	_ = catRow.Scan(&detail.Category)

	if fastPath() {
		rows, err := twdb.QueryContext(ctx, `SELECT id, product_id, rating, body FROM reviews WHERE product_id = ? ORDER BY id`, detail.Id)
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("get reviews (fast): %w", err))
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

	span := traceway.StartSpan(ctx, "load_reviews")
	idRows, err := twdb.QueryContext(ctx, `SELECT id FROM reviews WHERE product_id = ?`, detail.Id)
	if err != nil {
		span.End()
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("list review ids: %w", err))
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

	for _, rid := range reviewIds {
		slowJitter(150, 300)
		var rv Review
		rrow := twdb.QueryRowContext(ctx, `SELECT id, product_id, rating, body FROM reviews WHERE id = ?`, rid)
		if err := rrow.Scan(&rv.Id, &rv.ProductId, &rv.Rating, &rv.Body); err == nil {
			detail.Reviews = append(detail.Reviews, rv)
		}
	}
	span.End()

	rec := traceway.StartSpan(ctx, "recommendations.fetch")
	slowJitter(300, 700)
	rec.End()

	detail.ReviewCount = len(detail.Reviews)
	c.JSON(http.StatusOK, detail)
}
