{{- define "twic.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "twic.fullname" -}}
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

{{- define "twic.labels" -}}
helm.sh/chart: {{ .Chart.Name }}-{{ .Chart.Version | replace "+" "_" }}
app.kubernetes.io/name: {{ include "twic.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{- define "twic.selectorLabels" -}}
app.kubernetes.io/name: {{ include "twic.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{- define "twic.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "twic.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{- define "twic.webhookCertSecretName" -}}
{{- if and .Values.webhook.enabled (not .Values.webhook.certManager.enabled) (ne .Values.webhook.tls.secretName "") }}
{{- .Values.webhook.tls.secretName }}
{{- else }}
{{- printf "%s-webhook-certs" (include "twic.fullname" .) }}
{{- end }}
{{- end }}

{{- define "twic.webhookServiceName" -}}
{{- printf "%s-webhook" (include "twic.fullname" .) }}
{{- end }}
