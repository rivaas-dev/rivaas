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

  request_customer_id := split(input.user.id, ":")[3]
  input.customer.id == request_customer_id
}

# it's commented out because admin rule in admin.rego already covers this
#allow := true {
#  input.request.method == "GET"
#  input.request.path == "/v2/customers"
#
#  input.user.roles[_] == "administrator"  # only administrator have access
#}
