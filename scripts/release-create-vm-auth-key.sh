#!/bin/sh
set -eu

# This GitHub Actions-only boundary exchanges the job's OIDC identity for a
# narrowly scoped Tailscale API token, then prints one disposable VM key. Its
# caller must pipe stdout directly to the fixed native executor.
: "${ACTIONS_ID_TOKEN_REQUEST_URL:?GitHub OIDC request URL is required}"
: "${ACTIONS_ID_TOKEN_REQUEST_TOKEN:?GitHub OIDC request token is required}"
: "${SODA_TAILSCALE_CLIENT_ID:?Tailscale federated client ID is required}"
: "${SODA_TAILSCALE_AUDIENCE:?Tailscale federated audience is required}"

umask 077
temporary_dir=$(mktemp -d)
oidc_header=$temporary_dir/oidc-header
oidc_response=$temporary_dir/oidc-response.json
oidc_token=$temporary_dir/oidc-token
exchange_response=$temporary_dir/exchange-response.json
api_header=$temporary_dir/api-header
key_response=$temporary_dir/key-response.json

cleanup() {
    rm -f "$oidc_header" "$oidc_response" "$oidc_token" "$exchange_response" "$api_header" "$key_response"
    rmdir "$temporary_dir"
}
trap cleanup EXIT HUP INT TERM

printf 'Authorization: bearer %s\n' "$ACTIONS_ID_TOKEN_REQUEST_TOKEN" >"$oidc_header"
curl --fail --silent --show-error --header "@$oidc_header" \
    "$ACTIONS_ID_TOKEN_REQUEST_URL&audience=$SODA_TAILSCALE_AUDIENCE" >"$oidc_response"
jq -er '.value | strings | select(length > 0)' "$oidc_response" >"$oidc_token"

curl --fail --silent --show-error \
    --header 'Content-Type: application/x-www-form-urlencoded' \
    --data-urlencode "client_id=$SODA_TAILSCALE_CLIENT_ID" \
    --data-urlencode "jwt@$oidc_token" \
    https://api.tailscale.com/api/v2/oauth/token-exchange >"$exchange_response"
printf 'Authorization: Bearer %s\n' "$(jq -er '.access_token | strings | select(length > 0)' "$exchange_response")" >"$api_header"

curl --fail --silent --show-error \
    --header "@$api_header" \
    --header 'Content-Type: application/json' \
    --data '{"capabilities":{"devices":{"create":{"reusable":false,"ephemeral":true,"preauthorized":true,"tags":["tag:soda-ci-guest"]}}},"expirySeconds":3600,"description":"Soda OS release acceptance guest"}' \
    https://api.tailscale.com/api/v2/tailnet/-/keys >"$key_response"
jq -er '.key | strings | select(startswith("tskey-auth-"))' "$key_response"
