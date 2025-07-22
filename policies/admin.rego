# METADATA
# schemas:
#   - input.user.id: {type: string}
#   - input.user.roles: {type: [string]}
#   - input.request.method: {type: string}
#   - input.request.path: {type: string}
#   - input.key.actor_id: {type: string}
#   - input.key.creator_id: {type: string}
# custom:
#  id: 01H9J0WZ5WS7BVXVC6ATC1C0FW
package admin.api

default allow := false

# universal rule that covers all endpoints. Administrator have access to everything.
allow := true {
  input.user.roles[_] == "administrator"
}
