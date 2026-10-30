#!/bin/bash
# This script uses sed to replace static values with Helm template expressions.


set -e


# Usage: hack/helmify-easyauth-validating-webhook.sh <file>
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
sedi 's/^  name: validating-webhook-configuration/  name: {{ include "radix-easyauth.clusterScopedName" . }}/' "$file"

# Add labels and annotations after name
sedi '/^  name: {{ include "radix-easyauth.clusterScopedName" . }}/a\
  labels:\
    {{- include "radix-easyauth.labels" . | nindent 4 }}\
  annotations:\
    # cert-manager cainjector populates clientConfig.caBundle from this Certificate\
    cert-manager.io/inject-ca-from: {{ .Release.Namespace }}/{{ include "radix-easyauth.servingCertificateName" . }}' "$file"

# Replace service name and namespace
sedi 's/^      name: webhook-service/      name: {{ include "radix-easyauth.serviceName" . }}/' "$file"
sedi 's/^      namespace: system/      namespace: {{ .Release.Namespace }}/' "$file"

# Replace failurePolicy
sedi 's/^  failurePolicy: Fail/  failurePolicy: {{ .Values.webhook.failurePolicy }}/' "$file"

# Add scope after resources
sedi '/^    - radixapplications/a\
    scope: Namespaced' "$file"

# Add timeoutSeconds and namespaceSelector after sideEffects
sedi '/^  sideEffects: None/a\
  timeoutSeconds: {{ .Values.webhook.timeoutSeconds }}\
  namespaceSelector:\
    matchExpressions:\
    - key: kubernetes.io/metadata.name\
      operator: NotIn\
      values:\
      - {{ .Release.Namespace }}\
      {{- range .Values.webhook.excludedNamespaces }}\
      - {{ . }}\
      {{- end }}' "$file"

# Only installed when EasyAuth is enforced
sedi '1i\
{{- if .Values.config.enforced }}' "$file"
sedi '$a\
{{- end }}' "$file"
