#!/bin/sh
URL="${URL:-http://127.0.0.1:8090}"
curl "$URL/keys" -H "Content-Type: application/json" | jq .
