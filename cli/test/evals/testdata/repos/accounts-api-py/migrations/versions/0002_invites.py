"""invites

Revision ID: 0002
Revises: 0001
Create Date: 2026-02-17 14:40:02
"""
import sqlalchemy as sa
from alembic import op

revision = "0002"
down_revision = "0001"


def upgrade() -> None:
    op.create_table(
        "invites",
        sa.Column("id", sa.Integer, primary_key=True),
        sa.Column("org_id", sa.Integer, sa.ForeignKey("orgs.id"), nullable=False),
        sa.Column("email", sa.String(320), nullable=False),
        sa.Column("role", sa.String(32), nullable=False),
        sa.Column("token", sa.String(64), nullable=False, unique=True),
        sa.Column("invited_by", sa.Integer, sa.ForeignKey("users.id"), nullable=False),
        sa.Column("created_at", sa.DateTime, nullable=False),
        sa.Column("accepted_at", sa.DateTime, nullable=True),
    )
    op.create_index("ix_invites_org_id", "invites", ["org_id"])
