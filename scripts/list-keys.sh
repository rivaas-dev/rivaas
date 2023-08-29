#!/bin/sh
URL="${URL:-http://127.0.0.1:8080}"
curl "$URL/keys" -H "Content-Type: application/json" | jq .
