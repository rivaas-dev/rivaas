# METADATA
# schemas:
#   - input.user.id: {type: string}
#   - input.request.method: {type: string}
#   - input.request.path: {type: string}
#   - input.key.actor_id: {type: string}
#   - input.key.creator_id: {type: string}
# custom:
#  id: 01H9J0WZ5WS7BVXVC6ATC1C0FW
package admin.api

default allow := false

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
  requesterCustomerID := split(input.user.id, ":")[3]
  keyCustomerID := split(input.key.creator_id, ":")[3]
  input.request.method == "PATCH"
  input.request.path == "/keys/:id"
  requesterCustomerID == keyCustomerID
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

  requestCustomerID := split(input.user.id, ":")[3]
  keyCustomerID := split(input.key.actor_id, ":")[3]
  requestCustomerID == keyCustomerID
}

allow := true {
  input.request.method == "GET"
  input.request.path == "/v2/keys"

  # the response content is filtered out based on the customer id in the handler itself
}

allow := true {
  input.request.method == "POST"
  input.request.path == "/v2/keys"

  requestCustomerID := split(input.user.id, ":")[3]
  keyCustomerID := split(input.key.actor_id, ":")[3]
  requestCustomerID == keyCustomerID
}

allow := true {
  input.request.method == "PATCH"
  input.request.path == "/v2/keys/:id"

  requestCustomerID := split(input.user.id, ":")[3]
  keyCustomerID := split(input.key.actor_id, ":")[3]
  requestCustomerID == keyCustomerID
}

allow := true {
  input.request.method == "DELETE"
  input.request.path == "/v2/keys/:id"

  requestCustomerID := split(input.user.id, ":")[3]
  keyCustomerID := split(input.key.actor_id, ":")[3]
  requestCustomerID == keyCustomerID
}
