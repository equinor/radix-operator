{{/*
Expand the name of the chart.
*/}}
{{- define "radix-easyauth.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "radix-easyauth.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{- define "radix-easyauth.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "radix-easyauth.labels" -}}
helm.sh/chart: {{ include "radix-easyauth.chart" . }}
{{ include "radix-easyauth.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{- define "radix-easyauth.selectorLabels" -}}
app.kubernetes.io/name: {{ include "radix-easyauth.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{- define "radix-easyauth.serviceAccountName" -}}
{{- default (include "radix-easyauth.fullname" .) .Values.serviceAccount.name }}
{{- end }}

{{- define "radix-easyauth.serviceName" -}}
{{- printf "%s-webhook" (include "radix-easyauth.fullname" .) | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/* Bootstrap self-signed Issuer, only used to sign the CA certificate */}}
{{- define "radix-easyauth.selfSignedIssuerName" -}}
{{- printf "%s-selfsigned" (include "radix-easyauth.fullname" .) | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "radix-easyauth.caCertificateName" -}}
{{- printf "%s-ca" (include "radix-easyauth.fullname" .) | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/* CA Issuer used to sign the webhook serving certificate */}}
{{- define "radix-easyauth.webhookIssuerName" -}}
{{- printf "%s-webhook-issuer" (include "radix-easyauth.fullname" .) | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "radix-easyauth.servingCertificateName" -}}
{{- printf "%s-webhook-cert" (include "radix-easyauth.fullname" .) | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "radix-easyauth.servingCertificateSecretName" -}}
{{- printf "%s-webhook-tls" (include "radix-easyauth.fullname" .) | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Name for cluster scoped resources, includes namespace since the chart can be installed multiple times in a cluster
*/}}
{{- define "radix-easyauth.clusterScopedName" -}}
{{- printf "%s-%s" .Release.Namespace (include "radix-easyauth.fullname" .) | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Name of the release specific radix common config ConfigMap
*/}}
{{- define "radix-easyauth.commonConfigName" -}}
{{- printf "%s-config" (include "radix-easyauth.fullname" .) | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "radix-easyauth.commonConfigKey" -}}
configYaml
{{- end }}
