#!/usr/bin/env bash
# airbotix-set-tenant-fields.sh
#
# Set the 4 Airbotix-specific fields on an existing NewAPI user (= tenant in DeepRouter).
# Use this until docs/tasks/phase-1-admin-ui.md is done and the admin UI exposes the fields directly.
#
# Prerequisites:
#   - DeepRouter running via docker compose
#   - User already created via admin UI (see docs/tenant-onboarding.md steps 1-2)
#
# Usage:
#   bin/airbotix-set-tenant-fields.sh --username airbotix-kids --kids-mode --policy kid-safe \
#     --billing-webhook https://api.kidsinai.org/internal/deeprouter/billing
#
#   bin/airbotix-set-tenant-fields.sh --username jr-academy --policy adult \
#     --billing-webhook https://api.jiangren.com.au/_/deeprouter/billing
#
# Or just inspect without writing:
#   bin/airbotix-set-tenant-fields.sh --username airbotix-kids --dry-run

set -euo pipefail

# Defaults
COMPOSE_FILE=""
USERNAME=""
KIDS_MODE="false"
POLICY="passthrough"
WEBHOOK=""
PRICING_ID=""
DRY_RUN="false"

usage() {
  cat <<EOF_USAGE
Usage: $0 --username NAME [options]

Required:
  --username NAME              Existing NewAPI user to update

Optional:
  --kids-mode                  Set kids_mode = true (default: false)
  --policy PROFILE             policy_profile: passthrough | adult | kid-safe (default: passthrough)
  --billing-webhook URL        URL for per-request billing webhooks
  --custom-pricing-id ID       Custom pricing reference
  --compose-file PATH          Path to docker-compose.yml (auto-detected if omitted)
  --dry-run                    Print the SQL but do not execute
  -h, --help                   Show this help

Examples:
  $0 --username airbotix-kids --kids-mode --policy kid-safe \\
     --billing-webhook https://api.kidsinai.org/internal/deeprouter/billing

  $0 --username jr-academy --policy adult \\
     --billing-webhook https://api.jiangren.com.au/_/deeprouter/billing
EOF_USAGE
  exit 0
}

# Parse args
while [[ $# -gt 0 ]]; do
  case "$1" in
    --username) USERNAME="$2"; shift 2 ;;
    --kids-mode) KIDS_MODE="true"; shift ;;
    --policy) POLICY="$2"; shift 2 ;;
    --billing-webhook) WEBHOOK="$2"; shift 2 ;;
    --custom-pricing-id) PRICING_ID="$2"; shift 2 ;;
    --compose-file) COMPOSE_FILE="$2"; shift 2 ;;
    --dry-run) DRY_RUN="true"; shift ;;
    -h|--help) usage ;;
    *) echo "Unknown option: $1"; usage ;;
  esac
done

if [[ -z "$USERNAME" ]]; then
  echo "Error: --username is required" >&2
  usage
fi

case "$POLICY" in
  passthrough|adult|kid-safe) ;;
  *) echo "Error: --policy must be passthrough | adult | kid-safe (got '$POLICY')" >&2; exit 1 ;;
esac

# Auto-detect compose file
if [[ -z "$COMPOSE_FILE" ]]; then
  if [[ -f "docker-compose.yml" ]]; then
    COMPOSE_FILE="docker-compose.yml"
  elif [[ -f "docker-compose.dev.yml" ]]; then
    COMPOSE_FILE="docker-compose.dev.yml"
  else
    echo "Error: no docker-compose.yml found; specify with --compose-file" >&2
    exit 1
  fi
fi

# Build SQL
SQL="UPDATE users SET
  kids_mode = $KIDS_MODE,
  policy_profile = '$POLICY'$([[ -n "$WEBHOOK" ]] && echo ",
  billing_webhook_url = '$WEBHOOK'")$([[ -n "$PRICING_ID" ]] && echo ",
  custom_pricing_id = '$PRICING_ID'")
WHERE username = '$USERNAME'
RETURNING id, username, kids_mode, policy_profile, billing_webhook_url, custom_pricing_id;"

echo ""
echo "=== About to run (against $COMPOSE_FILE) ==="
echo "$SQL"
echo ""

if [[ "$DRY_RUN" == "true" ]]; then
  echo "(dry-run; not executed)"
  exit 0
fi

# Execute
docker compose -f "$COMPOSE_FILE" exec -T postgres \
  psql -U root -d new-api -c "$SQL"

echo ""
echo "✅ Done. Next steps:"
echo "  1. Have the tenant log in (or you, switched to the tenant user)"
echo "  2. Create a Token under that user (admin UI → Tokens → Add)"
echo "  3. Give the token's sk-... to the consumer product (e.g. kidsinai/platform-backend's DEEPROUTER_API_KEY)"
echo ""
echo "See docs/tenant-onboarding.md for the full flow."
