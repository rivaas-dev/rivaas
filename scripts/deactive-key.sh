#!/bin/sh
URL="${URL:-http://127.0.0.1:8090}"
[ -z "$ID" ] && echo "ID is needed" && exit 1
curl -X PATCH "$URL/keys/${ID}" -H "Content-Type: application/json" -d '
{
  "active": false
}' | jq .
