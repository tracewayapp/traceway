"""Request dependencies shared by the routes."""

from fastapi import Header


def current_account_id(x_account_id: int = Header()) -> int:
    return x_account_id
