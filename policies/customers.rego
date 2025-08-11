# METADATA
# schemas:
#   - input.user.id: {type: string}
#   - input.user.roles: {type: [string]}
#   - input.request.method: {type: string}
#   - input.request.path: {type: string}
#   - input.customer.id: {type: string}
# custom:
#  id: 01FNNVEFESY1SVFM5V261EPEMW
package admin.api

allow := true {
  input.request.method == "GET"
  input.request.path == "/v2/customers/:customerID"

  is_customer_authorized(input.user.id, input.customer.id, input.user.roles)
}

allow := true {
  input.request.method == "GET"
  input.request.path == "/v2/customers"

  is_admin(input.user.roles)  # only administrator have access
}
