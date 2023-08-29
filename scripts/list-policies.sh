#!/bin/sh
URL="${URL:-http://127.0.0.1:8080}"
curl "$URL/policies" -H "Content-Type: application/json" | jq .
