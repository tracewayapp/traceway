# checkout-api

Places orders for the storefront: `POST /api/orders` turns the shopper's cart
into an order, charges the card and queues the confirmation e-mail. Node.js,
Express, PostgreSQL (`orders.cart_id` is unique: one order per cart).
Telemetry goes to Traceway through the OpenTelemetry Node SDK
(`src/instrumentation.js`).

The order logic (`src/orders/service.js`) takes its repository and payment
client as arguments, so the tests run on Node's built-in test runner with an
in-memory repository and no dependencies installed:

```
node --test
npm start
```
