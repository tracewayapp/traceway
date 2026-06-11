import { captureMessage } from "@tracewayapp/frontend";
import { computeOrderTotal, pickFlashSaleItem, type CartItem } from "./pricing";

const demoCart: CartItem[] = [
  { sku: "SKU-001", quantity: 1, unitPriceCents: 1999 },
  { sku: "SKU-002", quantity: 3, unitPriceCents: 4999 },
];

function setStatus(text: string): void {
  const el = document.getElementById("status");
  if (el) {
    el.textContent = text;
  }
}

function handleCheckout(): void {
  setStatus("computing order total with corrupted discount rate...");
  const total = computeOrderTotal(demoCart, Number.NaN);
  setStatus(`total: ${total}`);
}

async function loadDiscountRate(): Promise<number> {
  await new Promise((resolve) => setTimeout(resolve, 10));
  throw new Error("discount service returned malformed payload");
}

function handleEvalThrow(): void {
  setStatus("throwing from inside eval...");
  eval("(function evalBoom() { throw new Error('boom from inside eval'); })()");
}

function handleForEachThrow(): void {
  setStatus("picking flash sale item...");
  const picked = pickFlashSaleItem(demoCart);
  setStatus(`picked: ${picked.sku}`);
}

function handleCaptureMessage(): void {
  setStatus("capturing stackless message...");
  captureMessage("manual message without a stack trace");
  setStatus("message captured");
}

document.getElementById("throw-nested")?.addEventListener("click", handleCheckout);

document.getElementById("throw-rejection")?.addEventListener("click", () => {
  setStatus("kicking off doomed async discount lookup...");
  void loadDiscountRate();
});

document.getElementById("throw-eval")?.addEventListener("click", handleEvalThrow);
document.getElementById("throw-foreach")?.addEventListener("click", handleForEachThrow);
document.getElementById("throw-message")?.addEventListener("click", handleCaptureMessage);
