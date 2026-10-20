'use strict';
const { request } = require('./http');

const env = process.env;
const CART = env.CART_URL || 'http://cart-service.internal:8080';
const CATALOG = env.CATALOG_URL || 'http://catalog-service.internal:8080';
const PRICING = env.PRICING_URL || 'http://pricing-service.internal:8080';
const INVENTORY = env.INVENTORY_URL || 'http://inventory-service.internal:8080';

const carts = {
  get: (cartId) => request(CART, 'GET', `/carts/${encodeURIComponent(cartId)}`),
};

const catalog = {
  get: (sku) => request(CATALOG, 'GET', `/products/${encodeURIComponent(sku)}`),
  // Up to 100 SKUs per call; unknown SKUs are left out of the response.
  getMany: (skus) => request(CATALOG, 'POST', '/products/batch', { skus }),
};

const pricing = {
  get: (sku, currency) => request(PRICING, 'GET', `/prices/${encodeURIComponent(sku)}?currency=${currency}`),
  // Up to 200 SKUs per call, one currency.
  getMany: (skus, currency) => request(PRICING, 'POST', '/prices/batch', { skus, currency }),
};

const inventory = {
  get: (sku) => request(INVENTORY, 'GET', `/stock/${encodeURIComponent(sku)}`),
  // Up to 200 SKUs per call.
  getMany: (skus) => request(INVENTORY, 'POST', '/stock/batch', { skus }),
};

module.exports = { carts, catalog, pricing, inventory };
