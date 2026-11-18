import { test } from "node:test";
import assert from "node:assert/strict";
import { createApp } from "./app.js";

async function withServer(fn) {
  const server = createApp().listen(0);
  await new Promise((resolve) => server.once("listening", resolve));
  const base = `http://127.0.0.1:${server.address().port}`;
  try {
    await fn(base);
  } finally {
    server.close();
  }
}

test("GET /api/orders/:id returns one order", async () => {
  await withServer(async (base) => {
    const res = await fetch(`${base}/api/orders/1002`);
    assert.equal(res.status, 200);
    assert.equal((await res.json()).customerId, "c-17");
  });
});

test("GET /api/orders filters by status", async () => {
  await withServer(async (base) => {
    const res = await fetch(`${base}/api/orders?status=pending`);
    assert.deepEqual((await res.json()).map((o) => o.id), ["1002", "1003"]);
  });
});

test("POST /api/orders validates the body", async () => {
  await withServer(async (base) => {
    const res = await fetch(`${base}/api/orders`, {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ customerId: "c-1", items: [] }),
    });
    assert.equal(res.status, 422);
  });
});
