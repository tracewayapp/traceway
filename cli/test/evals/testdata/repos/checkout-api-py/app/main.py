from fastapi import FastAPI

from app import cart, checkout

app = FastAPI(title="checkout-api")
app.include_router(cart.router)
app.include_router(checkout.router)


SHIPPING_OPTIONS = [
    {"id": "standard", "label": "Standard (3-5 days)", "priceCents": 0},
    {"id": "express", "label": "Express (1-2 days)", "priceCents": 990},
]


@app.get("/api/shipping/options")
def shipping_options():
    return SHIPPING_OPTIONS
