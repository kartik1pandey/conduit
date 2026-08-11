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
