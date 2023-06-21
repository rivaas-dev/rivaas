#!/bin/sh
URL="${URL:-http://127.0.0.1:8090}"
curl -sv -X POST "$URL/keys" -H "Content-Type: application/json" -d '
{
  "policies": [
    "test"
  ],
  "actor_id": "SF:1234567890",
  "description": "test Key",
  "quota_end_date": "2023-08-29",
  "quota": 30000,
  "contacts": {
    "emails": ["info@local.host"],
    "users": [1, 2, 3]
  }
}' | jq .
