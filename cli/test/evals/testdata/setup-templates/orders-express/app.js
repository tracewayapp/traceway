import express from "express";
import { createStore } from "./store.js";

export function createApp(store = createStore()) {
  const app = express();
  app.use(express.json());

  app.get("/api/orders", (req, res) => {
    res.json(store.list({ status: req.query.status }));
  });

  app.get("/api/orders/:id", (req, res) => {
    const order = store.get(req.params.id);
    if (!order) {
      return res.status(404).json({ error: "order not found" });
    }
    res.json(order);
  });

  app.post("/api/orders", (req, res) => {
    const { customerId, items } = req.body ?? {};
    if (!customerId || !Array.isArray(items) || items.length === 0) {
      return res.status(422).json({ error: "customerId and at least one item are required" });
    }
    res.status(201).json(store.create({ customerId, items }));
  });

  return app;
}
