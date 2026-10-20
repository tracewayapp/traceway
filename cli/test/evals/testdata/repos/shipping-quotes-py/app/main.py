import logging

from fastapi import FastAPI

from app.routes import quotes, tracking

logging.basicConfig(level=logging.INFO)

app = FastAPI(title="quotes-api")
app.include_router(quotes.router)
app.include_router(tracking.router)
