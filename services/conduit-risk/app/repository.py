"""Reads and writes scoring_events — conduit-risk's own history, used to
compute velocity/history features. See migrations/0001_init.sql for why this
table exists at all instead of querying conduit-core's data.
"""

from __future__ import annotations

from dataclasses import dataclass
from decimal import Decimal
from uuid import UUID

from psycopg_pool import ConnectionPool


@dataclass
class MerchantHistory:
    recent_event_count: int
    recent_total_amount: Decimal
    historical_avg_amount: Decimal | None


def merchant_history(
    pool: ConnectionPool, merchant_id: UUID, velocity_window_seconds: int
) -> MerchantHistory:
    with pool.connection() as conn:
        row = conn.execute(
            """
            SELECT
                COUNT(*) FILTER (WHERE created_at > now() - (%(window)s || ' seconds')::interval),
                COALESCE(SUM(amount) FILTER (
                    WHERE created_at > now() - (%(window)s || ' seconds')::interval
                ), 0),
                (SELECT AVG(amount) FROM scoring_events WHERE merchant_id = %(merchant_id)s)
            FROM scoring_events
            WHERE merchant_id = %(merchant_id)s
            """,
            {"window": velocity_window_seconds, "merchant_id": merchant_id},
        ).fetchone()
    count, recent_total, historical_avg = row
    return MerchantHistory(
        recent_event_count=count,
        recent_total_amount=Decimal(recent_total),
        historical_avg_amount=Decimal(historical_avg) if historical_avg is not None else None,
    )


@dataclass
class ScoringEvent:
    id: UUID
    payment_intent_id: UUID
    amount: Decimal
    currency: str
    risk_score: float
    decision: str
    stage: str
    reasons: list[str]
    created_at: object


def list_events(pool: ConnectionPool, merchant_id: UUID, limit: int) -> list[ScoringEvent]:
    with pool.connection() as conn:
        rows = conn.execute(
            """
            SELECT id, payment_intent_id, amount, currency, risk_score,
                   decision, stage, reasons, created_at
            FROM scoring_events
            WHERE merchant_id = %s
            ORDER BY created_at DESC
            LIMIT %s
            """,
            (merchant_id, limit),
        ).fetchall()
    return [
        ScoringEvent(
            id=row[0],
            payment_intent_id=row[1],
            amount=row[2],
            currency=row[3],
            risk_score=float(row[4]),
            decision=row[5],
            stage=row[6],
            reasons=row[7],
            created_at=row[8],
        )
        for row in rows
    ]


def record_event(
    pool: ConnectionPool,
    *,
    merchant_id: UUID,
    payment_intent_id: UUID,
    amount: Decimal,
    currency: str,
    risk_score: float,
    decision: str,
    stage: str,
    reasons: list[str],
) -> None:
    with pool.connection() as conn:
        conn.execute(
            """
            INSERT INTO scoring_events
                (merchant_id, payment_intent_id, amount, currency,
                 risk_score, decision, stage, reasons)
            VALUES (%s, %s, %s, %s, %s, %s, %s, %s)
            """,
            (
                merchant_id,
                payment_intent_id,
                amount,
                currency,
                round(risk_score, 4),
                decision,
                stage,
                reasons,
            ),
        )
        conn.commit()
