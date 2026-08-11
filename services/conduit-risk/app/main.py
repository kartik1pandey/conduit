from __future__ import annotations

from uuid import UUID

from fastapi import Depends, FastAPI, HTTPException

from app.authn import InternalAuth
from app.config import load_settings
from app.db import migrate, new_pool
from app.model import RiskModel
from app.policy import OPAClient, PolicyError
from app.repository import list_events
from app.schemas import RiskDecision, ScoreRequest, ScoreResponse
from app.scorer import Scorer


def create_app() -> FastAPI:
    settings = load_settings()
    pool = new_pool(settings.database_url)
    migrate(pool)

    model = RiskModel()
    opa = OPAClient(settings.opa_url)
    scorer = Scorer(pool, model, opa, settings)
    require_internal_auth = InternalAuth(settings.internal_jwt_secret)

    app = FastAPI(
        title="conduit-risk",
        description=(
            "Two-stage risk classifier (deterministic rules, then a trained "
            "model) with a real OPA/Rego policy layer deciding the "
            "score-to-decision mapping. Internal-only: every endpoint here "
            "verifies a short-lived JWT signed by conduit-core, never a "
            "merchant-facing credential."
        ),
        version="1.0",
    )

    @app.get("/health", tags=["health"], summary="Health check")
    def health() -> dict:
        """Returns 200 if both Postgres and the OPA sidecar are reachable, 503 otherwise."""
        try:
            with pool.connection() as conn:
                conn.execute("SELECT 1")
            db_ok = True
        except Exception:
            db_ok = False

        opa_ok = opa.healthy()

        if db_ok and opa_ok:
            return {"status": "ok"}
        raise HTTPException(status_code=503, detail="unavailable")

    @app.post(
        "/score",
        response_model=ScoreResponse,
        tags=["scoring"],
        summary="Score a payment intent",
    )
    def score(
        request: ScoreRequest, merchant_id: UUID = Depends(require_internal_auth)
    ) -> ScoreResponse:
        """Deterministic rules run first and can decline outright with no model
        call; otherwise a trained model produces a risk_score and a real OPA
        policy (policies/risk.rego) decides allow/decline from it. Every call
        is recorded in this service's own scoring_events history, which also
        feeds velocity/history features for future calls."""
        try:
            result = scorer.score(
                merchant_id, request.payment_intent_id, request.amount, request.currency
            )
        except PolicyError as exc:
            # OPA unreachable/misbehaving is a dependency outage, not a
            # scoring decision — surface it as a clean 503 rather than an
            # unhandled 500, matching how conduit-core reports a ledger-call
            # failure as a distinct, deliberate status rather than a decline.
            raise HTTPException(
                status_code=503, detail=f"policy engine unavailable: {exc}"
            ) from exc
        return ScoreResponse(
            payment_intent_id=request.payment_intent_id,
            decision=result.decision,
            risk_score=result.risk_score,
            stage=result.stage,
            reasons=result.reasons,
        )

    @app.get(
        "/v1/risk_decisions",
        response_model=list[RiskDecision],
        tags=["scoring"],
        summary="List risk decisions",
    )
    def risk_decisions(
        merchant_id: UUID = Depends(require_internal_auth), limit: int = 50
    ) -> list[RiskDecision]:
        """Lists this merchant's scoring history, newest first — backs
        conduit-dashboard's risk decisions view via conduit-core's proxy."""
        limit = max(1, min(limit, 200))
        events = list_events(pool, merchant_id, limit)
        return [
            RiskDecision(
                id=e.id,
                payment_intent_id=e.payment_intent_id,
                amount=e.amount,
                currency=e.currency,
                decision=e.decision,
                risk_score=e.risk_score,
                stage=e.stage,
                reasons=e.reasons,
                created_at=e.created_at,
            )
            for e in events
        ]

    return app


# Module-level instance for `uvicorn app.main:app`.
app = create_app()
