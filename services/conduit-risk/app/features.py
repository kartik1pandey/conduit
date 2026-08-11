"""Feature engineering shared by training/train_model.py and the live
scorer (app/model.py). This is the one place the feature vector's shape is
defined — both training and inference call build_feature_vector, so the
model always sees features assembled exactly the same way it was trained
on. Duplicating this logic between an offline training script and the
online service (even slightly differently) is exactly how real ML systems
get "train/serve skew" bugs — a model that scored well offline but behaves
oddly in production because production features aren't quite the ones it
learned from.
"""

from __future__ import annotations

import math
from decimal import Decimal

FEATURE_NAMES = [
    "amount",
    "log_amount",
    "hour_of_day",
    "recent_event_count",
    "recent_total_amount",
    "amount_vs_avg_ratio",
    "is_high_risk_currency",
]

# Arbitrary synthetic risk flags for this project's fake training data —
# not a real-world judgment about any currency or country.
HIGH_RISK_CURRENCIES = {"xof", "ngn", "byn"}


def build_feature_vector(
    *,
    amount: Decimal | float,
    currency: str,
    hour_of_day: int,
    recent_event_count: int,
    recent_total_amount: Decimal | float,
    historical_avg_amount: Decimal | float | None,
) -> list[float]:
    amount_f = float(amount)
    log_amount = math.log1p(amount_f)

    if historical_avg_amount and float(historical_avg_amount) > 0:
        amount_vs_avg_ratio = amount_f / float(historical_avg_amount)
    else:
        amount_vs_avg_ratio = 1.0  # no history yet — neutral, not risky by default

    is_high_risk_currency = 1.0 if currency.lower() in HIGH_RISK_CURRENCIES else 0.0

    return [
        amount_f,
        log_amount,
        float(hour_of_day),
        float(recent_event_count),
        float(recent_total_amount),
        amount_vs_avg_ratio,
        is_high_risk_currency,
    ]
