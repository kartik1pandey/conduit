from datetime import datetime
from decimal import Decimal
from typing import Literal
from uuid import UUID

from pydantic import BaseModel, Field


class ScoreRequest(BaseModel):
    # merchant_id deliberately isn't a field here — it comes from the
    # verified internal JWT (see authn.py), never from a client-controlled
    # body, the same rule every other internal endpoint in this project
    # follows.
    payment_intent_id: UUID
    amount: Decimal = Field(gt=0)
    currency: str


class ScoreResponse(BaseModel):
    payment_intent_id: UUID
    decision: Literal["allow", "decline"]
    risk_score: float
    stage: Literal["rules", "model"]
    reasons: list[str]


# RiskDecision is conduit-dashboard's read model for the "risk decisions
# with reasons" view (docs/ARCHITECTURE.md) — the same fields ScoreResponse
# returns at scoring time, plus the id and timestamp a list view needs and
# minus payment_intent_id's role as the single-record lookup key.
class RiskDecision(BaseModel):
    id: UUID
    payment_intent_id: UUID
    amount: Decimal
    currency: str
    decision: Literal["allow", "decline"]
    risk_score: float
    stage: Literal["rules", "model"]
    reasons: list[str]
    created_at: datetime
