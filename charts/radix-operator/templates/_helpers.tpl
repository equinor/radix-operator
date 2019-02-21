{{- define "imagePullSecret" }}
{{- printf "{\"auths\": {\"%s\": {\"auth\": \"%s\"}}}" .Values.imageCredentials.registry (printf "%s:%s" .Values.imageCredentials.username .Values.imageCredentials.password | b64enc) | b64enc }}
{{- end }}

{{- define "radix-sp-acr-azure-secret" }}
{{- printf "{\"name\": \"%s\", \"id\": \"%s\", \"password\": \"%s\", \"tenantId\": \"%s\"}" .Values.cicdacr.name .Values.cicdacr.id .Values.cicdacr.password .Values.cicdacr.tenantId | b64enc }}
{{- end }}