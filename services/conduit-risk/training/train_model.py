"""Trains the stage-2 model on training/dataset.csv and saves it to
models/risk_model.joblib.

LogisticRegression, not a gradient-boosted ensemble: this is a risk *score*
feeding a separate policy decision (see policies/risk.rego), not the final
decision-maker, and a linear model's coefficients are directly inspectable —
"why did this look risky" has a one-line answer (which features had large
positive weight), which matters for an auditable risk system in a way it
wouldn't for a pure Kaggle-leaderboard accuracy exercise. A boosted-tree
ensemble would very likely score a few points higher on this synthetic data
and is the right call the day interpretability stops mattering more than
that last bit of accuracy.
"""

from __future__ import annotations

from pathlib import Path

import joblib
import numpy as np
import pandas as pd
from sklearn.linear_model import LogisticRegression
from sklearn.metrics import classification_report, roc_auc_score
from sklearn.model_selection import train_test_split
from sklearn.pipeline import Pipeline
from sklearn.preprocessing import StandardScaler

from app.features import FEATURE_NAMES, build_feature_vector

RNG_SEED = 42


def load_features(df: pd.DataFrame) -> tuple[np.ndarray, np.ndarray]:
    rows = []
    for row in df.itertuples():
        historical_avg = None if pd.isna(row.historical_avg_amount) else row.historical_avg_amount
        rows.append(
            build_feature_vector(
                amount=row.amount,
                currency=row.currency,
                hour_of_day=row.hour_of_day,
                recent_event_count=row.recent_event_count,
                recent_total_amount=row.recent_total_amount,
                historical_avg_amount=historical_avg,
            )
        )
    return np.array(rows), df["label"].to_numpy()


def main() -> None:
    dataset_path = Path(__file__).parent / "dataset.csv"
    df = pd.read_csv(dataset_path)

    X, y = load_features(df)
    X_train, X_test, y_train, y_test = train_test_split(
        X, y, test_size=0.25, random_state=RNG_SEED, stratify=y
    )

    pipeline = Pipeline(
        [
            ("scaler", StandardScaler()),
            ("model", LogisticRegression(class_weight="balanced", random_state=RNG_SEED)),
        ]
    )
    pipeline.fit(X_train, y_train)

    test_probabilities = pipeline.predict_proba(X_test)[:, 1]
    test_predictions = (test_probabilities >= 0.5).astype(int)

    print(f"Feature order: {FEATURE_NAMES}")
    print(f"AUC: {roc_auc_score(y_test, test_probabilities):.4f}")
    print(classification_report(y_test, test_predictions, target_names=["legit", "fraud"]))

    coefficients = pipeline.named_steps["model"].coef_[0]
    print("Coefficients (on standardized features):")
    for name, coef in zip(FEATURE_NAMES, coefficients):
        print(f"  {name:24s} {coef:+.3f}")

    out_path = Path(__file__).parent.parent / "models" / "risk_model.joblib"
    out_path.parent.mkdir(exist_ok=True)
    joblib.dump(pipeline, out_path)
    print(f"saved model to {out_path}")


if __name__ == "__main__":
    main()
