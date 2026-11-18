"""Builds the account page payload from the rows the queries return."""


def build_summary(account, order_count, recent, spend_by_month):
    return {
        "account": {
            "id": account["id"],
            "email": account["email"],
            "displayName": account["display_name"],
            "plan": account["plan"],
            "memberSince": account["created_at"].date().isoformat(),
        },
        "orderCount": order_count,
        "recentOrders": [
            {"id": o["id"], "status": o["status"], "totalCents": o["total_cents"], "createdAt": o["created_at"].isoformat()}
            for o in recent
        ],
        "spendByMonth": [
            {"month": m["month"].strftime("%Y-%m"), "orders": m["orders"], "spendCents": int(m["spend_cents"] or 0)}
            for m in spend_by_month
        ],
    }
