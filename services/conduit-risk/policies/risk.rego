package conduit.risk

import rego.v1

# Score vs. decision, kept deliberately separate: app/model.py's job is only
# to produce a risk_score (a probability estimate); this policy's job is to
# decide what to do about a given score. A risk analyst can review and
# change the threshold here without touching Python, redeploying the model,
# or understanding how it was trained — the same separation
# docs/ARCHITECTURE.md calls for reusing again in Phase 4 for dashboard RBAC.

default decision := "allow"

decision := "decline" if {
	input.risk_score >= threshold
}

# Lower bar for currencies this project's synthetic training data treats as
# higher-risk (see app/features.py) — a merchant transacting in one of these
# gets declined at a lower score than one transacting in, say, USD.
threshold := 0.75 if {
	lower(input.currency) in high_risk_currencies
}

threshold := 0.85 if {
	not lower(input.currency) in high_risk_currencies
}

high_risk_currencies := {"xof", "ngn", "byn"}
