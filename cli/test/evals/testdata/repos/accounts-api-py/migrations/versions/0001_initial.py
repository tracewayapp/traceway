"""users, orgs and memberships

Revision ID: 0001
Revises:
Create Date: 2025-11-03 09:12:44
"""
import sqlalchemy as sa
from alembic import op

revision = "0001"
down_revision = None


def upgrade() -> None:
    op.create_table(
        "orgs",
        sa.Column("id", sa.Integer, primary_key=True),
        sa.Column("name", sa.String(200), nullable=False),
        sa.Column("plan", sa.String(32), nullable=False, server_default="free"),
    )
    op.create_table(
        "users",
        sa.Column("id", sa.Integer, primary_key=True),
        sa.Column("email", sa.String(320), nullable=False),
        sa.Column("display_name", sa.String(200), nullable=False),
        sa.Column("org_id", sa.Integer, sa.ForeignKey("orgs.id"), nullable=False),
        sa.Column("created_at", sa.DateTime, server_default=sa.func.now(), nullable=False),
    )
    op.create_index("ix_users_org_id", "users", ["org_id"])
    op.create_table(
        "org_members",
        sa.Column("org_id", sa.Integer, sa.ForeignKey("orgs.id"), primary_key=True),
        sa.Column("user_id", sa.Integer, sa.ForeignKey("users.id"), primary_key=True),
        sa.Column("role", sa.String(32), nullable=False, server_default="member"),
    )
