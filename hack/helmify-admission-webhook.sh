#!/bin/bash
# Script to helmify .temp-dir/admission-webhook.yaml to match .temp-dir/radix-webhook-configuration.yaml
# This script uses sed to replace static values with Helm template expressions.


set -e


# Usage: hack/helmify-admission-webhook.sh <file>
if [ "$#" -ne 1 ]; then
  echo "Usage: $0 <file>"
  exit 1
fi

file="$1"


# Remove all lines that are just '---'
sed -i '/^---$/d' "$file"

# Insert Helm if at the top
sed -i '1s;^;{{ if .Values.radixWebhook.enabled }}\n\n;' "$file"

# Replace metadata name
sed -i 's/^  name: validating-webhook-configuration/  name: {{ .Values.radixWebhook.webhookConfigurationName }}/' "$file"

# Replace service name, namespace, and add port
sed -i 's/^      name: webhook-service/      name: {{ template "radix-webhook.fullname" . }}/' "$file"
sed -i 's/^      namespace: system/      namespace: {{ .Release.Namespace }}/' "$file"
sed -i '/^      path: \/radix\/v1\/radixregistration\/validation/a \\      port: {{ .Values.radixWebhook.port}}' "$file"

# Replace webhook name
sed -i 's/^  name: validate.radixapplication.radix.equinor.com/  name: radixregistrations.validation.radix.equinor.com/' "$file"

# Add matchPolicy after failurePolicy
sed -i '/^  failurePolicy: Fail/a \\  matchPolicy: Equivalent' "$file"

# Add an empty line before the last {{ end }}
echo '' >> "$file"
echo '{{ end }}' >> "$file"
