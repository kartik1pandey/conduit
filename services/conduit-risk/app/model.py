"""Loads the trained stage-2 model once at startup and scores live requests
through it. See training/train_model.py for how the model was produced —
build_feature_vector (app/features.py) is the shared contract that keeps
this file's inputs identical in shape to what the model was trained on.
"""

from __future__ import annotations

from decimal import Decimal
from pathlib import Path

import joblib

from app.features import build_feature_vector

MODEL_PATH = Path(__file__).parent.parent / "models" / "risk_model.joblib"


class RiskModel:
    def __init__(self, model_path: Path = MODEL_PATH):
        self._pipeline = joblib.load(model_path)

    def score(
        self,
        *,
        amount: Decimal,
        currency: str,
        hour_of_day: int,
        recent_event_count: int,
        recent_total_amount: Decimal,
        historical_avg_amount: Decimal | None,
    ) -> float:
        features = build_feature_vector(
            amount=amount,
            currency=currency,
            hour_of_day=hour_of_day,
            recent_event_count=recent_event_count,
            recent_total_amount=recent_total_amount,
            historical_avg_amount=historical_avg_amount,
        )
        probability = self._pipeline.predict_proba([features])[0][1]
        return float(probability)
