"""Generates a synthetic labeled transaction dataset for training the
stage-2 model.

There's no real historical fraud data in this project (no real merchants,
no real transactions) — this generates one that's internally consistent: a
latent "true" fraud probability is computed from a hand-written generative
rule (fraud-prone merchants, unusual-hour transactions, amount spikes,
high-risk currencies, bursty velocity), and each row's label is a noisy
Bernoulli sample from that probability, not the rule's raw output. That gap
between "the true generating rule" and "the noisy label the model actually
sees" is deliberate — it's what makes this a genuine (if small) learning
problem instead of the model just memorizing the rule back.

Deterministic: a fixed RNG seed means re-running this script reproduces the
exact same dataset.csv, which is what makes Checkpoint 3.1 ("scoring a known
sample input returns the same decision") a meaningful, repeatable claim
rather than one that depends on whichever random data happened to be
generated on some previous run.
"""

from __future__ import annotations

from collections import deque
from datetime import datetime, timedelta
from pathlib import Path

import numpy as np
import pandas as pd

RNG_SEED = 42
N_MERCHANTS = 200
TRANSACTIONS_PER_MERCHANT = 30
VELOCITY_WINDOW_SECONDS = 60
FRAUD_PRONE_MERCHANT_RATE = 0.08

CURRENCIES = ["usd", "usd", "usd", "eur", "gbp", "xof", "ngn"]
HIGH_RISK_CURRENCIES = {"xof", "ngn", "byn"}


def generate(rng: np.random.Generator) -> pd.DataFrame:
    rows = []

    for merchant_idx in range(N_MERCHANTS):
        is_fraud_prone = rng.random() < FRAUD_PRONE_MERCHANT_RATE
        base_amount = float(rng.lognormal(mean=3.0, sigma=0.8))  # ~$20 typical
        currency = str(rng.choice(CURRENCIES))

        recent_window: deque[tuple[datetime, float]] = deque()
        history: list[float] = []
        t = datetime(2026, 1, 1)

        for _ in range(TRANSACTIONS_PER_MERCHANT):
            if is_fraud_prone and rng.random() < 0.3:
                dt_seconds = rng.uniform(1, 5)  # a burst
            else:
                dt_seconds = rng.uniform(30, 3600)
            t = t + timedelta(seconds=float(dt_seconds))

            while (
                recent_window
                and (t - recent_window[0][0]).total_seconds() > VELOCITY_WINDOW_SECONDS
            ):
                recent_window.popleft()

            recent_event_count = len(recent_window)
            recent_total_amount = sum(a for _, a in recent_window)
            historical_avg_amount = float(np.mean(history)) if history else None

            amount_multiplier = float(rng.lognormal(mean=0, sigma=0.5))
            if is_fraud_prone and rng.random() < 0.4:
                amount_multiplier *= float(rng.uniform(3, 10))  # occasional large spike
            amount = round(base_amount * amount_multiplier, 2)

            hour_of_day = t.hour

            # The latent generating rule — never seen directly by the model,
            # only through the noisy label sampled from it below.
            risk_logit = -4.0
            risk_logit += 2.5 if is_fraud_prone else 0.0
            risk_logit += 0.15 * recent_event_count
            risk_logit += 1.5 if amount > base_amount * 5 else 0.0
            risk_logit += 1.0 if currency in HIGH_RISK_CURRENCIES else 0.0
            risk_logit += 0.8 if hour_of_day < 5 else 0.0
            true_fraud_probability = 1.0 / (1.0 + np.exp(-risk_logit))
            label = int(rng.random() < true_fraud_probability)

            rows.append(
                {
                    "merchant_idx": merchant_idx,
                    "amount": amount,
                    "currency": currency,
                    "hour_of_day": hour_of_day,
                    "recent_event_count": recent_event_count,
                    "recent_total_amount": recent_total_amount,
                    "historical_avg_amount": historical_avg_amount,
                    "label": label,
                }
            )

            recent_window.append((t, amount))
            history.append(amount)

    return pd.DataFrame(rows)


def main() -> None:
    rng = np.random.default_rng(RNG_SEED)
    df = generate(rng)
    out_path = Path(__file__).parent / "dataset.csv"
    df.to_csv(out_path, index=False)
    print(f"wrote {len(df)} rows to {out_path} (fraud rate: {df['label'].mean():.3f})")


if __name__ == "__main__":
    main()
