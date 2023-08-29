#!/bin/sh
URL="${URL:-http://127.0.0.1:8080}"
EXPIRES_AT=$(date --date="2 days" "+%Y-%m-%d")
curl -sv -X POST "$URL/keys" -H "Content-Type: application/json" -H "X-Customer-ID: urn:online:user:123:456" -d @- <<EOF | jq .
{
  "policies": [
    "policy-1"
  ],
  "actor_id": "SF:1234567890",
  "customer_id": "01234",
  "account_id": "56789",
  "description": "test Key",
  "expires_at": "${EXPIRES_AT}",
  "quota": 30000,
  "contacts": {
    "emails": ["info@local.host"],
    "users": [1, 2, 3]
  },
  "rate_limit": {
    "rate": 1000,
    "per": 1000
  },
  "environment": "production",
  "labels": {
    "department": "finance",
    "code": "123",
    "labelo": "blabla"
  }
}
EOF
