package main

import "database/sql"

func initDB() {
	var err error
	db, err = sql.Open("sqlite3", ":memory:")
	if err != nil {
		panic(err)
	}
	db.SetMaxOpenConns(1)

	schema := []string{
		`CREATE TABLE categories (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL
		)`,
		`CREATE TABLE products (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			description TEXT NOT NULL,
			price_cents INTEGER NOT NULL,
			image_url TEXT NOT NULL,
			stock INTEGER NOT NULL,
			category_id INTEGER NOT NULL
		)`,
		`CREATE TABLE reviews (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			product_id INTEGER NOT NULL,
			rating INTEGER NOT NULL,
			body TEXT NOT NULL
		)`,
		`CREATE TABLE coupons (
			code TEXT PRIMARY KEY,
			percent_off INTEGER NOT NULL,
			active INTEGER NOT NULL
		)`,
		`CREATE TABLE cart_items (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			product_id INTEGER NOT NULL,
			qty INTEGER NOT NULL
		)`,
	}
	for _, stmt := range schema {
		if _, err := db.Exec(stmt); err != nil {
			panic(err)
		}
	}

	seed()
}

func mustExec(query string, args ...any) {
	if _, err := db.Exec(query, args...); err != nil {
		panic(err)
	}
}

func seed() {
	categories := []struct {
		id   int
		name string
	}{
		{1, "Apparel"},
		{2, "Electronics"},
		{3, "Home"},
		{4, "Books"},
	}
	for _, c := range categories {
		mustExec(`INSERT INTO categories (id, name) VALUES (?, ?)`, c.id, c.name)
	}

	products := []struct {
		name       string
		desc       string
		priceCents int
		stock      int
		categoryId int
	}{
		{"Classic Tee", "Soft combed-cotton t-shirt", 1999, 120, 1},
		{"Denim Jacket", "Vintage wash denim jacket", 8999, 30, 1},
		{"Running Sneakers", "Lightweight everyday runners", 6499, 45, 1},
		{"Wireless Earbuds", "Noise-cancelling earbuds", 12999, 60, 2},
		{"Mechanical Keyboard", "Hot-swappable RGB keyboard", 10999, 25, 2},
		{"4K Monitor", "27-inch UHD display", 29999, 15, 2},
		{"USB-C Hub", "7-in-1 aluminium hub", 3999, 80, 2},
		{"Scented Candle", "Cedar and sage soy candle", 1499, 200, 3},
		{"Ceramic Mug", "Hand-glazed 12oz mug", 1299, 150, 3},
		{"Throw Blanket", "Chunky knit wool blend", 4999, 40, 3},
		{"Go in Practice", "Pragmatic patterns for Go services", 3499, 70, 4},
		{"The Pragmatic Programmer", "A classic on software craft", 3999, 55, 4},
	}
	for _, p := range products {
		mustExec(`INSERT INTO products (name, description, price_cents, image_url, stock, category_id) VALUES (?, ?, ?, ?, ?, ?)`,
			p.name, p.desc, p.priceCents, "", p.stock, p.categoryId)
	}
	mustExec(`UPDATE products SET image_url = 'https://picsum.photos/seed/twshop' || id || '/320/200'`)

	bodies := []string{
		"Exactly as described, very happy.",
		"Decent quality for the price.",
		"Shipping was slow but the product is great.",
		"Would buy again.",
		"Not quite what I expected, but okay.",
		"Five stars, highly recommend.",
	}
	for productId := 1; productId <= len(products); productId++ {
		n := 2 + (productId % 4)
		for i := 0; i < n; i++ {
			rating := 3 + ((productId + i) % 3)
			body := bodies[(productId+i)%len(bodies)]
			mustExec(`INSERT INTO reviews (product_id, rating, body) VALUES (?, ?, ?)`, productId, rating, body)
		}
	}

	coupons := []struct {
		code    string
		percent int
		active  int
	}{
		{"SAVE10", 10, 1},
		{"HALF50", 50, 1},
		{"EXPIRED", 25, 0},
	}
	for _, cp := range coupons {
		mustExec(`INSERT INTO coupons (code, percent_off, active) VALUES (?, ?, ?)`, cp.code, cp.percent, cp.active)
	}
}
