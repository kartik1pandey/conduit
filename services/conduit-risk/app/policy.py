"""Client for the OPA sidecar that evaluates policies/risk.rego. conduit-risk
never embeds policy logic in Python — every score-to-decision mapping is
Rego, evaluated by a real OPA server process, not a Python re-implementation
of "what OPA would do." See docs/learning for why this is worth the extra
moving part.
"""

from __future__ import annotations

import httpx


class PolicyError(Exception):
    pass


class OPAClient:
    def __init__(self, base_url: str, timeout: float = 2.0):
        self._client = httpx.Client(base_url=base_url, timeout=timeout)

    def decide(self, *, risk_score: float, currency: str) -> str:
        try:
            response = self._client.post(
                "/v1/data/conduit/risk/decision",
                json={"input": {"risk_score": risk_score, "currency": currency}},
            )
            response.raise_for_status()
        except httpx.HTTPError as exc:
            raise PolicyError(f"OPA request failed: {exc}") from exc

        result = response.json().get("result")
        if result not in ("allow", "decline"):
            raise PolicyError(f"unexpected OPA decision: {result!r}")
        return result

    def healthy(self) -> bool:
        try:
            response = self._client.get("/health")
            return response.status_code == 200
        except httpx.HTTPError:
            return False
