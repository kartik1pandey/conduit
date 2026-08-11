package conduit.risk

import rego.v1

test_low_risk_usd_is_allowed if {
	decision == "allow" with input as {"risk_score": 0.2, "currency": "usd"}
}

test_high_risk_usd_is_declined if {
	decision == "decline" with input as {"risk_score": 0.9, "currency": "usd"}
}

test_usd_at_exact_threshold_is_declined if {
	decision == "decline" with input as {"risk_score": 0.85, "currency": "usd"}
}

test_usd_just_below_threshold_is_allowed if {
	decision == "allow" with input as {"risk_score": 0.84, "currency": "usd"}
}

test_high_risk_currency_declined_at_lower_threshold if {
	decision == "decline" with input as {"risk_score": 0.76, "currency": "ngn"}
}

test_high_risk_currency_allowed_below_its_lower_threshold if {
	decision == "allow" with input as {"risk_score": 0.74, "currency": "ngn"}
}

test_currency_matching_is_case_insensitive if {
	decision == "decline" with input as {"risk_score": 0.76, "currency": "NGN"}
}
