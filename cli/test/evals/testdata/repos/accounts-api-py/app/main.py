from fastapi import FastAPI

from app import invites, users

app = FastAPI(title="accounts-api")
app.include_router(users.router)
app.include_router(invites.router)
