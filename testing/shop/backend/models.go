package main

type Product struct {
	Id          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	PriceCents  int    `json:"price_cents"`
	ImageUrl    string `json:"image_url"`
	Stock       int    `json:"stock"`
	CategoryId  int    `json:"category_id"`
	Category    string `json:"category"`
	ReviewCount int    `json:"review_count"`
}

type Review struct {
	Id        int    `json:"id"`
	ProductId int    `json:"product_id"`
	Rating    int    `json:"rating"`
	Body      string `json:"body"`
}

type ProductDetail struct {
	Product
	Reviews []Review `json:"reviews"`
}

type CartLine struct {
	Id         int    `json:"id"`
	ProductId  int    `json:"product_id"`
	Name       string `json:"name"`
	PriceCents int    `json:"price_cents"`
	ImageUrl   string `json:"image_url"`
	Qty        int    `json:"qty"`
	LineTotal  int    `json:"line_total_cents"`
}

type AddToCartRequest struct {
	ProductId int `json:"product_id"`
	Qty       int `json:"qty"`
}

type CouponRequest struct {
	Code string `json:"code"`
}

type CheckoutRequest struct {
	Name      string `json:"name"`
	Email     string `json:"email"`
	CardLast4 string `json:"card_last4"`
}
