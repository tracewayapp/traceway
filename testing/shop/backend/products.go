package main

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

func listProducts(c *gin.Context) {
	ctx := c.Request.Context()

	if fastPath() {
		rows, err := db.QueryContext(ctx, `
			SELECT p.id, p.name, p.description, p.price_cents, p.image_url, p.stock, p.category_id,
			       c.name AS category, COUNT(r.id) AS review_count
			FROM products p
			LEFT JOIN categories c ON c.id = p.category_id
			LEFT JOIN reviews r ON r.product_id = p.id
			GROUP BY p.id
			ORDER BY p.id`)
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("list products (fast): %w", err))
			return
		}
		products := []Product{}
		for rows.Next() {
			var p Product
			if err := rows.Scan(&p.Id, &p.Name, &p.Description, &p.PriceCents, &p.ImageUrl, &p.Stock, &p.CategoryId, &p.Category, &p.ReviewCount); err != nil {
				rows.Close()
				c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("scan product (fast): %w", err))
				return
			}
			products = append(products, p)
		}
		rows.Close()
		c.JSON(http.StatusOK, products)
		return
	}

	rows, err := db.QueryContext(ctx, `SELECT id, name, description, price_cents, image_url, stock, category_id FROM products ORDER BY id`)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("list products: %w", err))
		return
	}
	products := []Product{}
	for rows.Next() {
		var p Product
		if err := rows.Scan(&p.Id, &p.Name, &p.Description, &p.PriceCents, &p.ImageUrl, &p.Stock, &p.CategoryId); err != nil {
			rows.Close()
			c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("scan product: %w", err))
			return
		}
		products = append(products, p)
	}
	rows.Close()

	for i := range products {
		slowJitter(15, 50)
		catRow := db.QueryRowContext(ctx, `SELECT name FROM categories WHERE id = ?`, products[i].CategoryId)
		_ = catRow.Scan(&products[i].Category)

		countRow := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM reviews WHERE product_id = ?`, products[i].Id)
		_ = countRow.Scan(&products[i].ReviewCount)
	}

	c.JSON(http.StatusOK, products)
}

func getProduct(c *gin.Context) {
	ctx := c.Request.Context()
	id := c.Param("id")

	var detail ProductDetail
	row := db.QueryRowContext(ctx, `SELECT id, name, description, price_cents, image_url, stock, category_id FROM products WHERE id = ?`, id)
	if err := row.Scan(&detail.Id, &detail.Name, &detail.Description, &detail.PriceCents, &detail.ImageUrl, &detail.Stock, &detail.CategoryId); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "product not found"})
			return
		}
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("get product: %w", err))
		return
	}

	catRow := db.QueryRowContext(ctx, `SELECT name FROM categories WHERE id = ?`, detail.CategoryId)
	_ = catRow.Scan(&detail.Category)

	if fastPath() {
		rows, err := db.QueryContext(ctx, `SELECT id, product_id, rating, body FROM reviews WHERE product_id = ? ORDER BY id`, detail.Id)
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("get reviews (fast): %w", err))
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

	idRows, err := db.QueryContext(ctx, `SELECT id FROM reviews WHERE product_id = ?`, detail.Id)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("list review ids: %w", err))
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
		slowJitter(10, 30)
		var rv Review
		rrow := db.QueryRowContext(ctx, `SELECT id, product_id, rating, body FROM reviews WHERE id = ?`, rid)
		if err := rrow.Scan(&rv.Id, &rv.ProductId, &rv.Rating, &rv.Body); err == nil {
			detail.Reviews = append(detail.Reviews, rv)
		}
	}

	slowJitter(80, 200)

	detail.ReviewCount = len(detail.Reviews)
	c.JSON(http.StatusOK, detail)
}
