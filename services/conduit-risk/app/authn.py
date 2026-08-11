"""Verifies the short-lived internal JWT conduit-core signs on every
service-to-service call — the same contract every other service's
internal/authn package implements in Go. conduit-risk only ever trusts a
merchant_id that arrived via a verified JWT, never one supplied directly in
a request body.
"""

from __future__ import annotations

from uuid import UUID

import jwt
from fastapi import Header, HTTPException


class InternalAuth:
    def __init__(self, secret: str):
        self._secret = secret

    def __call__(self, authorization: str = Header(default="")) -> UUID:
        if not authorization.startswith("Bearer "):
            raise HTTPException(status_code=401, detail="missing bearer token")
        token = authorization.removeprefix("Bearer ")

        try:
            claims = jwt.decode(token, self._secret, algorithms=["HS256"])
        except jwt.InvalidTokenError:
            raise HTTPException(status_code=401, detail="invalid internal token") from None

        merchant_id = claims.get("merchant_id")
        if not merchant_id:
            raise HTTPException(status_code=401, detail="missing merchant_id claim")
        try:
            return UUID(merchant_id)
        except ValueError:
            raise HTTPException(status_code=401, detail="invalid merchant_id claim") from None
