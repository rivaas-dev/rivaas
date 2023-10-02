#!/bin/sh
URL="${URL:-http://127.0.0.1:8090}"
[ -z "$ID" ] && echo "ID is needed" && exit 1
http patch "$URL/keys/${ID}" "Content-Type:application/json" <<EOF
{
  "active": false
}
EOF
