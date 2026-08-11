# conduit-risk

Risk scoring: a fast deterministic rules pre-filter, then a trained
scikit-learn model producing a `risk_score`, then an OPA/Rego policy
deciding `allow`/`decline` from that score. See
[`docs/learning`](../../docs/learning) for why this project builds its own
risk engine here rather than wrapping a pre-existing one, and for the full
design rationale.

## Running locally

Needs Postgres and a real OPA server (this project uses OPA for real — see
`policies/risk.rego` — not a Python re-implementation of what OPA would do).

```bash
# from the repo root
docker compose up -d postgres opa

cd services/conduit-risk
python -m venv .venv && .venv/Scripts/activate    # or source .venv/bin/activate on macOS/Linux
pip install -r requirements.txt

export RISK_DATABASE_URL=postgres://conduit:conduit@localhost:5432/conduit_risk
export INTERNAL_JWT_SECRET=local-dev-secret-not-real
export OPA_URL=http://localhost:8181

uvicorn app.main:app --reload --port 8001
```

## Testing

```bash
export RISK_DATABASE_URL=postgres://conduit:conduit@localhost:5432/conduit_risk
export INTERNAL_JWT_SECRET=local-dev-secret-not-real
export OPA_URL=http://localhost:8181
python -m pytest tests/
```

`tests/test_rules.py` and `tests/test_model.py` need no external
dependencies. `tests/test_api.py` (including the Checkpoint 3.1
golden-sample regression cases) is skipped, not failed, if
`RISK_DATABASE_URL`/`OPA_URL` aren't set.

## Retraining the model

```bash
python -m training.generate_dataset   # writes training/dataset.csv (deterministic, fixed seed)
python -m training.train_model        # writes models/risk_model.joblib
```

`app/features.py` is the single source of truth for feature engineering,
imported by both the training script and the live scorer — training and
inference are guaranteed to build feature vectors identically.

## API

Requires `Authorization: Bearer <internal JWT>`, signed by conduit-core.

| Method | Path | Description |
|---|---|---|
| `GET` | `/health` | 200 if Postgres and OPA are both reachable, 503 otherwise |
| `POST` | `/score` | Scores a payment intent; returns `decision`, `risk_score`, `stage`, and `reasons` |
