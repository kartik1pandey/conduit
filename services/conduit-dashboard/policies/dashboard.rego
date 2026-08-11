package conduit.dashboard

import rego.v1

# RBAC for conduit-dashboard's role model (owner/developer/read-only) —
# docs/ARCHITECTURE.md calls for reusing the same OPA/Rego pattern
# conduit-risk's policies/risk.rego already established, rather than a
# separate bespoke authorization system. This runs against its own OPA
# instance (services/conduit-dashboard/policies), not conduit-risk's — each
# service stays self-contained per ARCHITECTURE.md's repo layout, so
# dashboard's authorization policy lives in dashboard's own directory even
# though the mechanism (a real OPA server, not a hand-rolled if/else) is
# identical.
#
# input shape: {"role": "owner" | "developer" | "read-only", "action": "view" | "refund" | "invite_user" | "create_payment" | "manage_webhooks"}

valid_roles := {"owner", "developer", "read-only"}

default allow := false

# Every valid role can view merchant-scoped data (transactions, webhook
# deliveries, risk decisions) — "read-only" describes what a role can't
# mutate, not that it can't see anything at all.
allow if {
	input.role in valid_roles
	input.action == "view"
}

# Refund is a real money-affecting action (Checkpoint 4.3's exact
# verification target: a read-only session must get 403 here). Owner and
# developer can trigger one; read-only cannot.
allow if {
	input.role in {"owner", "developer"}
	input.action == "refund"
}

# Creating and confirming a payment intent, and registering a webhook
# endpoint, are both mutating, money-adjacent actions — same tier as
# refund, same rule: owner and developer can, read-only cannot.
allow if {
	input.role in {"owner", "developer"}
	input.action == "create_payment"
}

allow if {
	input.role in {"owner", "developer"}
	input.action == "manage_webhooks"
}

# Inviting a new dashboard user (and choosing their role) is an
# account-management action reserved for the owner — a developer granting
# themselves or a colleague broader access would be a privilege-escalation
# path, not a convenience.
allow if {
	input.role == "owner"
	input.action == "invite_user"
}
