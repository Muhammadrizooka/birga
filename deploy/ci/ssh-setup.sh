#!/usr/bin/env bash
# Prepares SSH on a GitHub Actions runner: installs the deploy key and trusts the server only
# if its ED25519 host key matches SERVER_HOST_FINGERPRINT.
# Env: SSH_KEY, SSH_HOST, SERVER_HOST_FINGERPRINT.
set -euo pipefail
: "${SSH_KEY:?SERVER_SSH_KEY secret is missing}"
: "${SSH_HOST:?SERVER_HOST secret is missing}"
: "${SERVER_HOST_FINGERPRINT:?SERVER_HOST_FINGERPRINT is missing}"

install -d -m 700 ~/.ssh
printf '%s\n' "$SSH_KEY" >~/.ssh/id_ed25519
chmod 600 ~/.ssh/id_ed25519

hostkey="$(ssh-keyscan -t ed25519 "$SSH_HOST" 2>/dev/null)"
actual="$(ssh-keygen -lf - <<<"$hostkey" | awk '{ print $2 }')"
if [[ "$actual" != "$SERVER_HOST_FINGERPRINT" ]]; then
	echo "::error::The server's host key ($actual) does not match SERVER_HOST_FINGERPRINT. Not connecting."
	exit 1
fi
printf '%s\n' "$hostkey" >>~/.ssh/known_hosts
echo "Server host key verified"
