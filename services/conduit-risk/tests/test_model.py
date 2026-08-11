from decimal import Decimal

from app.model import RiskModel


def test_model_loads_and_scores():
    model = RiskModel()
    score = model.score(
        amount=Decimal("25.00"),
        currency="usd",
        hour_of_day=14,
        recent_event_count=0,
        recent_total_amount=Decimal(0),
        historical_avg_amount=Decimal("20.00"),
    )
    assert 0.0 <= score <= 1.0


def test_model_scores_obviously_risky_pattern_higher_than_clean_pattern():
    model = RiskModel()

    clean_score = model.score(
        amount=Decimal("20.00"),
        currency="usd",
        hour_of_day=14,
        recent_event_count=0,
        recent_total_amount=Decimal(0),
        historical_avg_amount=Decimal("20.00"),
    )

    risky_score = model.score(
        amount=Decimal("500.00"),  # far above historical average
        currency="ngn",  # synthetic high-risk currency
        hour_of_day=3,  # late night
        recent_event_count=12,  # high velocity
        recent_total_amount=Decimal("2000.00"),
        historical_avg_amount=Decimal("20.00"),
    )

    assert risky_score > clean_score
