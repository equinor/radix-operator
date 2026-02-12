#!/bin/bash
# This script uses sed to replace static values with Helm template expressions.


set -e


# Usage: hack/helmify-admission-webhook.sh <file>
if [ "$#" -ne 1 ]; then
  echo "Usage: $0 <file>"
  exit 1
fi

file="$1"

# Cross-platform in-place sed (BSD vs GNU)
sedi() {
  if [[ "$(uname)" == "Darwin" ]]; then
    sed -i '' "$@"
  else
    sed -i "$@"
  fi
}

# Remove all lines that are just '---'
sedi '/^---$/d' "$file"

# Replace metadata name
sedi 's/^  name: validating-webhook-configuration/  name: radix-webhook-configuration/' "$file"


# Replace service name, namespace, and add port
sedi 's/^      name: webhook-service/      name: radix-webhook/' "$file"
sedi 's/^      namespace: system/      namespace: {{ .Release.Namespace }}/' "$file"
sedi '/^      path: \/radix\/v1\/radixregistration\/validation/a\
      port: 443' "$file"
sedi '/^      path: \/radix\/v1\/radixapplication\/validation/a\
      port: 443' "$file"

# Add matchPolicy after failurePolicy
sedi '/^  failurePolicy: Fail/a\
  matchPolicy: Equivalent' "$file"
