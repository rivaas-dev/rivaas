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
#  id: 01H9J0WZ5WS7BVXVC6ATC1C0FW
package admin.api

# v1 endpoints

allow := true {
  input.request.method == "GET"
  input.request.path == "/keys/:id"
}

allow := true {
  input.request.method == "GET"
  input.request.path == "/keys"
}

allow := true {
  input.request.method == "POST"
  input.request.path == "/keys"
}

allow := true {
  requester_customer_id := split(input.user.id, ":")[3]
  key_customer_id := split(input.key.creator_id, ":")[3]
  input.request.method == "PATCH"
  input.request.path == "/keys/:id"
  requester_customer_id == key_customer_id
}

allow := true {
  input.request.method == "DELETE"
  input.request.path == "/keys/:id"
  input.user.id == input.key.creator_id
}

# v2 endpoints

allow := true {
  input.request.method == "GET"
  input.request.path == "/v2/keys/:id"

  is_user_authorized(input.user.id, input.key.actor_id, input.user.roles)
}

allow := true {
  input.request.method == "GET"
  input.request.path == "/v2/keys"

  # the response content is filtered out based on the customer id in the handler itself
}

allow := true {
  input.request.method == "POST"
  input.request.path == "/v2/keys"

  is_user_authorized(input.user.id, input.key.actor_id, input.user.roles)
  input.user.number_of_keys.current < input.user.number_of_keys.max
}

allow := true {
  input.request.method == "PATCH"
  input.request.path == "/v2/keys/:id"

  is_user_authorized(input.user.id, input.key.actor_id, input.user.roles)
}

allow := true {
  input.request.method == "DELETE"
  input.request.path == "/v2/keys/:id"

  is_user_authorized(input.user.id, input.key.actor_id, input.user.roles)
}
