# METADATA
# schemas:
#   - input.user.id: {type: string}
#   - input.user.roles: {type: [string]}
#   - input.user.number_of_keys.current: {type: int}
#   - input.user.number_of_keys.max: {type: int}
#   - input.request.method: {type: string}
#   - input.request.path: {type: string}
#   - input.key.actor_id: {type: string}
#   - input.key.creator_id: {type: string}
# custom:
#  id: 0000023H83FCTZZSE007713WA3
package admin.api

default allow := false

# -------------------------------------------------------------
# Helpers
# -------------------------------------------------------------

# helper: is the caller an administrator?
is_admin(roles) {
  some i
  roles[i] == "administrator"
}

# helped: a user is authorized if:
# (a) they're an admin
is_user_authorized(request_actor, key_actor, roles) {
  is_admin(roles)
}

# (b) they're a regular user and their customer_id matches the customer_id of the key
is_user_authorized(request_actor, key_actor, roles) {
  not is_admin(roles)
  requester_customer_id := split(request_actor, ":")[3]
  key_customer_id       := split(key_actor,     ":")[3]
  requester_customer_id == key_customer_id
}

# helper: a customer is authorized if:
# (a) they're an admin
is_customer_authorized(request_actor, customer_id, roles) {
  is_admin(roles)
}

# (b) they're a regular user and their customer_id matches the customer_id of the key
is_customer_authorized(request_actor, customer_id, roles) {
  not is_admin(roles)
  requester_customer_id := split(request_actor, ":")[3]
  requester_customer_id == customer_id
}


