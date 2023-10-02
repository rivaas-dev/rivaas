#!/bin/sh
URL="${URL:-http://127.0.0.1:8090}"
http "$URL/policies" "Content-Type:application/json"
