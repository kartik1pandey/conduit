"""API-level tests, including the Checkpoint 3.1 golden-sample regression
test: scoring a known input must keep returning the same decision AEGIS
(this project's own risk engine — see docs/learning for why there's no
external classifier to compare against) originally produced on it.

Skipped, not failed, when the real dependencies aren't configured — the
same pattern every Go service's integration tests use for
LEDGER_DATABASE_URL/REDIS_URL. Requires a real Postgres (RISK_DATABASE_URL)
and a real OPA server (OPA_URL) actually running, not mocks — this is
integration-testing the real wiring, including the real Rego policy.
"""

from __future__ import annotations

import os
import uuid

import pytest

if not os.environ.get("RISK_DATABASE_URL") or not os.environ.get("OPA_URL"):
    pytest.skip(
        "RISK_DATABASE_URL/OPA_URL not set; skipping integration test", allow_module_level=True
    )

from fastapi.testclient import TestClient  # noqa: E402

from app.main import app  # noqa: E402


@pytest.fixture
def client():
    return TestClient(app)


def score(
    client: TestClient, token: str, amount: str, currency: str, payment_intent_id=None
) -> dict:
    resp = client.post(
        "/score",
        headers={"Authorization": f"Bearer {token}"},
        json={
            "payment_intent_id": str(payment_intent_id or uuid.uuid4()),
            "amount": amount,
            "currency": currency,
        },
    )
    assert resp.status_code == 200, resp.text
    return resp.json()


def test_score_requires_auth(client):
    resp = client.post(
        "/score",
        json={"payment_intent_id": str(uuid.uuid4()), "amount": "10.00", "currency": "usd"},
    )
    assert resp.status_code == 401


# --- Checkpoint 3.1: golden-sample regression -------------------------------
#
# Each case's merchant_id is unique and never reused elsewhere, so a fresh
# merchant has no scoring history — these decisions only depend on the
# input itself, not on state accumulated by other tests running first.


def test_golden_sample_small_clean_amount_is_allowed_by_model(client, sign_internal_jwt):
    merchant_id = uuid.UUID("a0000000-0000-0000-0000-000000000001")
    result = score(client, sign_internal_jwt(merchant_id), "25.00", "usd")

    assert result["decision"] == "allow"
    assert result["stage"] == "model"
    assert result["reasons"] == []
    # The exact float can shift by a negligible amount across platforms/
    # scikit-learn versions; what Checkpoint 3.1 actually requires reproduced
    # is the *decision*, which has a wide margin here (well under the 0.85
    # USD policy threshold).
    assert 0.1 < result["risk_score"] < 0.4


def test_golden_sample_amount_over_hard_ceiling_is_declined_by_rules(client, sign_internal_jwt):
    merchant_id = uuid.UUID("a0000000-0000-0000-0000-000000000002")
    result = score(client, sign_internal_jwt(merchant_id), "10000.01", "usd")

    assert result == {
        "payment_intent_id": result["payment_intent_id"],
        "decision": "decline",
        "risk_score": 1.0,
        "stage": "rules",
        "reasons": ["amount_exceeds_hard_limit"],
    }


def test_golden_sample_large_amount_high_risk_currency_is_declined_by_model(
    client, sign_internal_jwt
):
    merchant_id = uuid.UUID("a0000000-0000-0000-0000-000000000003")
    result = score(client, sign_internal_jwt(merchant_id), "5000.00", "ngn")

    assert result["decision"] == "decline"
    assert result["stage"] == "model"
    assert result["reasons"] == ["risk_score_above_policy_threshold"]
    assert result["risk_score"] > 0.99


def test_golden_sample_velocity_limit_is_declined_by_rules_after_repeated_requests(
    client, sign_internal_jwt
):
    merchant_id = uuid.UUID("a0000000-0000-0000-0000-000000000004")
    token = sign_internal_jwt(merchant_id)

    for _ in range(10):
        score(client, token, "10.00", "usd")

    result = score(client, token, "10.00", "usd")
    assert result["decision"] == "decline"
    assert result["stage"] == "rules"
    assert result["reasons"] == ["velocity_limit_exceeded"]


# --- /v1/risk_decisions: conduit-dashboard's read view ----------------------


def test_risk_decisions_requires_auth(client):
    resp = client.get("/v1/risk_decisions")
    assert resp.status_code == 401


def test_risk_decisions_lists_this_merchants_history_newest_first(client, sign_internal_jwt):
    merchant_id = uuid.UUID("a0000000-0000-0000-0000-000000000005")
    token = sign_internal_jwt(merchant_id)

    score(client, token, "25.00", "usd")
    score(client, token, "10000.01", "usd")  # declined by rules

    resp = client.get("/v1/risk_decisions", headers={"Authorization": f"Bearer {token}"})
    assert resp.status_code == 200, resp.text
    decisions = resp.json()

    assert len(decisions) == 2
    # newest first: the hard-ceiling decline was scored second
    assert decisions[0]["decision"] == "decline"
    assert decisions[0]["stage"] == "rules"
    assert decisions[1]["decision"] == "allow"


def test_risk_decisions_never_returns_another_merchants_history(client, sign_internal_jwt):
    merchant_a = uuid.UUID("a0000000-0000-0000-0000-000000000006")
    merchant_b = uuid.UUID("a0000000-0000-0000-0000-000000000007")

    score(client, sign_internal_jwt(merchant_a), "25.00", "usd")

    resp = client.get(
        "/v1/risk_decisions", headers={"Authorization": f"Bearer {sign_internal_jwt(merchant_b)}"}
    )
    assert resp.status_code == 200, resp.text
    assert resp.json() == []
