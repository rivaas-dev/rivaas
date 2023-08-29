#!/bin/sh
URL="${URL:-http://127.0.0.1:8080}"
[ -z "$ID" ] && echo "ID is needed" && exit 1
curl -v -X DELETE "$URL/keys/${ID}" -H "Content-Type: application/json"
