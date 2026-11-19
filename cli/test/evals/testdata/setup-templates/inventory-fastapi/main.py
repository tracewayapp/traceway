from fastapi import FastAPI, HTTPException
from pydantic import BaseModel, Field

app = FastAPI(title="inventory-api", version="2.3.1")


class Item(BaseModel):
    id: int
    sku: str
    name: str
    on_hand: int


class StockAdjustment(BaseModel):
    delta: int = Field(..., description="Units added (positive) or removed (negative)")
    reason: str


ITEMS: dict[int, Item] = {
    1: Item(id=1, sku="MUG-01", name="Enamel mug", on_hand=140),
    2: Item(id=2, sku="TEE-03", name="Logo tee", on_hand=32),
    3: Item(id=3, sku="CAP-02", name="Dad cap", on_hand=0),
}


@app.get("/api/items")
def list_items(in_stock: bool = False) -> list[Item]:
    items = list(ITEMS.values())
    if in_stock:
        items = [i for i in items if i.on_hand > 0]
    return items


@app.get("/api/items/{item_id}")
def get_item(item_id: int) -> Item:
    item = ITEMS.get(item_id)
    if item is None:
        raise HTTPException(status_code=404, detail="item not found")
    return item


@app.post("/api/items/{item_id}/adjustments")
def adjust_stock(item_id: int, adjustment: StockAdjustment) -> Item:
    item = get_item(item_id)
    if item.on_hand + adjustment.delta < 0:
        raise HTTPException(status_code=422, detail="stock cannot go below zero")
    item.on_hand += adjustment.delta
    return item
