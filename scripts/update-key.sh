#!/bin/sh
URL="${URL:-http://127.0.0.1:8090}"
[ -z "$ID" ] && echo "ID is needed" && exit 1
curl -X PATCH "$URL/keys/${ID}" -H "Content-Type: application/json" -d '
{
  "description": "updated test key",
  "quota": 10000,
  "contacts": {
    "emails": ["support@local.host"],
    "users": [4, 5]
  },
  "rate_limit": {
    "rate": 300,
    "per": 1000
  }
}' | jq .
