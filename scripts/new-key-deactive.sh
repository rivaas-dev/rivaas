#!/bin/sh
URL="${URL:-http://127.0.0.1:8090}"
EXPIRES_AT=$(date --date="2 days" "+%Y-%m-%d")
http post "$URL/keys" "Content-Type:application/json" <<EOF
{
  "policies": [
    "policy-1"
  ],
  "actor_id": "SF:1234567890",
  "description": "test Key",
  "expires_at": "${EXPIRES_AT}",
  "quota": 30000,
  "contacts": {
    "emails": ["info@local.host"],
    "users": [1, 2, 3]
  },
  "active": false
}
EOF
