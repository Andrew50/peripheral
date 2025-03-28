#!/usr/bin/env bash
set -Eeuo pipefail

ENVIRONMENT="${1:-}"

echo "Loading environment config for: $ENVIRONMENT"

# Example: If you keep environment-level config in config/<env>/env.sh, you could source it:
# if [[ -f "config/${ENVIRONMENT}/env.sh" ]]; then
#   source "config/${ENVIRONMENT}/env.sh"
# else
#   echo "No environment-specific config found for $ENVIRONMENT"
# fi

# For now, just a placeholder
echo "Environment config loaded."
