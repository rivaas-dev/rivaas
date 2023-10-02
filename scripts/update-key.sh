#!/bin/sh
URL="${URL:-http://127.0.0.1:8090}"
[ -z "$ID" ] && echo "ID is needed" && exit 1
EXPIRES_AT=$(date --date="3 days" "+%Y-%m-%d")
http patch "$URL/keys/${ID}" "Content-Type:application/json" <<EOF
{
  "description": "updated test key",
  "quota": 10000,
  "expires_at": "${EXPIRES_AT}",
  "contacts": {
    "emails": ["support@local.host"],
    "users": [4, 5]
  },
  "rate_limit": {
    "rate": 300,
    "per": 1000
  },
  "policies": [
    "policy-2"
  ],
  "labels": {
    "new": "label"
  }
}
EOF
