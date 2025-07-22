# METADATA
# schemas:
#   - input.user.id: {type: string}
#   - input.user.roles: {type: [string]}
#   - input.request.method: {type: string}
#   - input.request.path: {type: string}
#   - input.customer.id: {type: string}
#   - input.customer.account.id: {type: string}
# custom:
#  id: 01JWRKT0SS2GZ6CN80667KJRDG
package admin.api

allow := true {
  input.request.method == "PUT"
  input.request.path == "/v2/accounts/:accountID"

  request_customer_id := split(input.user.id, ":")[3]
  input.customer.id == request_customer_id
}

allow := true {
  input.request.method == "GET"
  input.request.path == "/v2/customers/:customerID/accounts"

  request_customer_id := split(input.user.id, ":")[3]
  input.customer.id == request_customer_id
}
