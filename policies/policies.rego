# METADATA
# schemas:
#   - input.user.roles: {type: [string]}
#   - input.request.method: {type: string}
#   - input.request.path: {type: string}
# custom:
#  id: 01KCPB7AZX9F557FS2058Z1WZT
package admin.api

allow := true {
  input.request.method == "GET"
  input.request.path == "/v2/policies"

  is_admin(input.user.roles)
}