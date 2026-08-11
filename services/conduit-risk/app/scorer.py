"""Orchestrates a single scoring request: stage 1 rules -> (if not already
declined) stage 2 model -> OPA policy -> record the outcome in this
service's own history.
"""

from __future__ import annotations

from dataclasses import dataclass
from datetime import UTC, datetime
from decimal import Decimal
from uuid import UUID

from psycopg_pool import ConnectionPool

from app import rules
from app.config import Settings
from app.model import RiskModel
from app.policy import OPAClient
from app.repository import merchant_history, record_event


@dataclass
class ScoreResult:
    decision: str
    risk_score: float
    stage: str
    reasons: list[str]


class Scorer:
    def __init__(self, pool: ConnectionPool, model: RiskModel, opa: OPAClient, settings: Settings):
        self._pool = pool
        self._model = model
        self._opa = opa
        self._settings = settings

    def score(
        self, merchant_id: UUID, payment_intent_id: UUID, amount: Decimal, currency: str
    ) -> ScoreResult:
        history = merchant_history(self._pool, merchant_id, self._settings.velocity_window_seconds)

        rule_result = rules.evaluate(
            amount,
            history,
            hard_decline_amount_ceiling=self._settings.hard_decline_amount_ceiling,
            velocity_max_requests=self._settings.velocity_max_requests,
        )

        if rule_result.declined:
            # Stage 1 already has a verdict — the model and the policy
            # engine never run at all for this request.
            result = ScoreResult(
                decision="decline", risk_score=1.0, stage="rules", reasons=rule_result.reasons
            )
        else:
            now = datetime.now(UTC)
            risk_score = self._model.score(
                amount=amount,
                currency=currency,
                hour_of_day=now.hour,
                recent_event_count=history.recent_event_count,
                recent_total_amount=history.recent_total_amount,
                historical_avg_amount=history.historical_avg_amount,
            )
            decision = self._opa.decide(risk_score=risk_score, currency=currency)
            reasons = [] if decision == "allow" else ["risk_score_above_policy_threshold"]
            result = ScoreResult(
                decision=decision, risk_score=risk_score, stage="model", reasons=reasons
            )

        record_event(
            self._pool,
            merchant_id=merchant_id,
            payment_intent_id=payment_intent_id,
            amount=amount,
            currency=currency,
            risk_score=result.risk_score,
            decision=result.decision,
            stage=result.stage,
            reasons=result.reasons,
        )
        return result
