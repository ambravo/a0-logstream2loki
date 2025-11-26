#!/usr/bin/env python3
"""
Helper script to generate HMAC-SHA256 bearer tokens for authentication

Usage:
    python3 generate-token.py <tenant> <secret>

Example:
    python3 generate-token.py amba my-secret-key
"""

import sys
import hmac
import hashlib


def generate_token(tenant: str, secret: str) -> str:
    """Generate HMAC-SHA256 token for the given tenant and secret"""
    return hmac.new(
        secret.encode(),
        tenant.encode(),
        hashlib.sha256
    ).hexdigest()


def main():
    if len(sys.argv) != 3:
        print("Usage: python3 generate-token.py <tenant> <secret>")
        print("")
        print("Example:")
        print("  python3 generate-token.py amba my-secret-key")
        sys.exit(1)

    tenant = sys.argv[1]
    secret = sys.argv[2]

    token = generate_token(tenant, secret)

    print(f"Tenant: {tenant}")
    print(f"Token:  {token}")
    print("")
    print("Use in Authorization header:")
    print(f"  Authorization: Bearer {token}")
    print("")
    print("Example curl command:")
    print(f'  curl -X POST "http://localhost:8080/logs?tenant={tenant}" \\')
    print(f'    -H "Authorization: Bearer {token}" \\')
    print('    -H "Content-Type: application/x-ndjson" \\')
    print('    --data-binary @example-log.jsonl')


if __name__ == "__main__":
    main()
