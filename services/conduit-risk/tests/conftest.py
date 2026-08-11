from __future__ import annotations

import os
from datetime import UTC, datetime, timedelta
from uuid import UUID

import jwt
import pytest

TEST_JWT_SECRET = os.environ.get("INTERNAL_JWT_SECRET", "test-internal-secret")


@pytest.fixture
def sign_internal_jwt():
    def _sign(merchant_id: UUID) -> str:
        claims = {
            "merchant_id": str(merchant_id),
            "exp": datetime.now(UTC) + timedelta(minutes=1),
        }
        return jwt.encode(claims, TEST_JWT_SECRET, algorithm="HS256")

    return _sign
