#!/bin/bash
#
# Helper script to generate HMAC-SHA256 bearer tokens for authentication
#
# Usage:
#   ./generate-token.sh <tenant> <secret>
#
# Example:
#   ./generate-token.sh amba my-secret-key
#

if [ $# -ne 2 ]; then
    echo "Usage: $0 <tenant> <secret>"
    echo ""
    echo "Example:"
    echo "  $0 amba my-secret-key"
    exit 1
fi

TENANT="$1"
SECRET="$2"

# Generate HMAC-SHA256 token
TOKEN=$(echo -n "$TENANT" | openssl dgst -sha256 -hmac "$SECRET" | cut -d' ' -f2)

echo "Tenant: $TENANT"
echo "Token:  $TOKEN"
echo ""
echo "Use in Authorization header:"
echo "  Authorization: Bearer $TOKEN"
echo ""
echo "Example curl command:"
echo "  curl -X POST \"http://localhost:8080/logs?tenant=$TENANT\" \\"
echo "    -H \"Authorization: Bearer $TOKEN\" \\"
echo "    -H \"Content-Type: application/x-ndjson\" \\"
echo "    --data-binary @example-log.jsonl"
