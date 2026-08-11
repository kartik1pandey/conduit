"""Stage 1: deterministic, cheap pre-filter rules.

Real fraud systems run a fast rule layer in front of any model specifically
so obviously-bad (or obviously-fine) traffic never pays for an expensive
model inference — a decline here short-circuits before stage 2 (the model)
or the OPA policy call ever run. This stage is intentionally simple and
auditable: every rule is a plain, testable Python function, not something
buried inside a trained model's weights.
"""

from __future__ import annotations

from dataclasses import dataclass
from decimal import Decimal

from app.repository import MerchantHistory


@dataclass
class RuleResult:
    declined: bool
    reasons: list[str]


def evaluate(
    amount: Decimal,
    history: MerchantHistory,
    *,
    hard_decline_amount_ceiling: float,
    velocity_max_requests: int,
) -> RuleResult:
    reasons: list[str] = []

    if amount > Decimal(str(hard_decline_amount_ceiling)):
        reasons.append("amount_exceeds_hard_limit")

    if history.recent_event_count >= velocity_max_requests:
        reasons.append("velocity_limit_exceeded")

    return RuleResult(declined=bool(reasons), reasons=reasons)
