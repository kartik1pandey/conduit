package conduit.dashboard

import rego.v1

# --- view: every valid role can view ---------------------------------------

test_owner_can_view if {
	allow with input as {"role": "owner", "action": "view"}
}

test_developer_can_view if {
	allow with input as {"role": "developer", "action": "view"}
}

test_read_only_can_view if {
	allow with input as {"role": "read-only", "action": "view"}
}

# --- refund: the literal Checkpoint 4.3 verification target ----------------

test_owner_can_refund if {
	allow with input as {"role": "owner", "action": "refund"}
}

test_developer_can_refund if {
	allow with input as {"role": "developer", "action": "refund"}
}

test_read_only_cannot_refund if {
	not allow with input as {"role": "read-only", "action": "refund"}
}

# --- create_payment / manage_webhooks: same tier as refund ------------------

test_owner_can_create_payment if {
	allow with input as {"role": "owner", "action": "create_payment"}
}

test_developer_can_create_payment if {
	allow with input as {"role": "developer", "action": "create_payment"}
}

test_read_only_cannot_create_payment if {
	not allow with input as {"role": "read-only", "action": "create_payment"}
}

test_owner_can_manage_webhooks if {
	allow with input as {"role": "owner", "action": "manage_webhooks"}
}

test_developer_can_manage_webhooks if {
	allow with input as {"role": "developer", "action": "manage_webhooks"}
}

test_read_only_cannot_manage_webhooks if {
	not allow with input as {"role": "read-only", "action": "manage_webhooks"}
}

# --- invite_user: owner-only, to prevent self/peer privilege escalation ----

test_owner_can_invite_user if {
	allow with input as {"role": "owner", "action": "invite_user"}
}

test_developer_cannot_invite_user if {
	not allow with input as {"role": "developer", "action": "invite_user"}
}

test_read_only_cannot_invite_user if {
	not allow with input as {"role": "read-only", "action": "invite_user"}
}

# --- defensive: an unrecognized role or action must never default-allow ----

test_unknown_role_cannot_refund if {
	not allow with input as {"role": "auditor", "action": "refund"}
}

test_unknown_action_is_denied if {
	not allow with input as {"role": "owner", "action": "delete_merchant"}
}
