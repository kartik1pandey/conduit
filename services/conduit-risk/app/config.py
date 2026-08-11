"""Configuration loaded from environment variables — no config file, no
flags, matching the pattern every Go service in this project uses (see e.g.
conduit-ledger/internal/config/config.go). pydantic-settings just gives the
same idea (env-var struct with validation) idiomatically for Python.
"""

from pydantic import Field
from pydantic_settings import BaseSettings


class Settings(BaseSettings):
    database_url: str = Field(validation_alias="RISK_DATABASE_URL")
    internal_jwt_secret: str = Field(validation_alias="INTERNAL_JWT_SECRET")
    opa_url: str = Field(validation_alias="OPA_URL")
    port: int = Field(default=8001, validation_alias="CONDUIT_RISK_PORT")

    # Stage 1 rule thresholds — deliberately code constants, not env vars:
    # these are the classifier's own documented behavior, not per-environment
    # tuning knobs. Changing them is a code change with a test to update,
    # the same way changing WEBHOOK_MAX_RETRIES's *meaning* would be.
    hard_decline_amount_ceiling: float = 10_000.00
    velocity_window_seconds: int = 60
    velocity_max_requests: int = 10


def load_settings() -> Settings:
    return Settings()
