from decimal import Decimal

from app.repository import MerchantHistory
from app.rules import evaluate

CLEAN_HISTORY = MerchantHistory(
    recent_event_count=0, recent_total_amount=Decimal(0), historical_avg_amount=None
)


def test_normal_amount_passes():
    result = evaluate(
        Decimal("49.99"),
        CLEAN_HISTORY,
        hard_decline_amount_ceiling=10_000.00,
        velocity_max_requests=10,
    )
    assert not result.declined
    assert result.reasons == []


def test_amount_over_ceiling_is_declined():
    result = evaluate(
        Decimal("10000.01"),
        CLEAN_HISTORY,
        hard_decline_amount_ceiling=10_000.00,
        velocity_max_requests=10,
    )
    assert result.declined
    assert "amount_exceeds_hard_limit" in result.reasons


def test_amount_at_exact_ceiling_passes():
    result = evaluate(
        Decimal("10000.00"),
        CLEAN_HISTORY,
        hard_decline_amount_ceiling=10_000.00,
        velocity_max_requests=10,
    )
    assert not result.declined


def test_velocity_limit_is_declined():
    busy_history = MerchantHistory(
        recent_event_count=10, recent_total_amount=Decimal(500), historical_avg_amount=Decimal(50)
    )
    result = evaluate(
        Decimal("10.00"),
        busy_history,
        hard_decline_amount_ceiling=10_000.00,
        velocity_max_requests=10,
    )
    assert result.declined
    assert "velocity_limit_exceeded" in result.reasons


def test_both_rules_can_fire_together():
    busy_history = MerchantHistory(
        recent_event_count=15, recent_total_amount=Decimal(0), historical_avg_amount=None
    )
    result = evaluate(
        Decimal("20000.00"),
        busy_history,
        hard_decline_amount_ceiling=10_000.00,
        velocity_max_requests=10,
    )
    assert result.declined
    assert set(result.reasons) == {"amount_exceeds_hard_limit", "velocity_limit_exceeded"}
