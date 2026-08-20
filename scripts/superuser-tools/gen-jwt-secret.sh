#!/usr/bin/env bash
# gen-jwt-secret.sh — generate a cryptographically random JWT_SECRET
# suitable for pasting into .env.
#
# Usage:
#   ./gen-jwt-secret.sh

set -euo pipefail

if command -v openssl >/dev/null 2>&1; then
  SECRET=$(openssl rand -base64 32)
elif command -v python3 >/dev/null 2>&1; then
  SECRET=$(python3 -c "import secrets; print(secrets.token_urlsafe(32))")
else
  echo "ERROR: need either openssl or python3 to generate a secret." >&2
  exit 1
fi

echo "$SECRET"
echo "" >&2
echo "Add this to your .env as:" >&2
echo "  JWT_SECRET=$SECRET" >&2
echo "" >&2
echo "⚠ Rotating this secret invalidates every currently-issued access token" >&2
echo "  — all logged-in users will need to log in again." >&2
