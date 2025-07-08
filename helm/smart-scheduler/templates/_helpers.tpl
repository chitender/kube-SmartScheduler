{{/*
Expand the name of the chart.
*/}}
{{- define "smart-scheduler.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "smart-scheduler.fullname" -}}
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

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "smart-scheduler.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "smart-scheduler.labels" -}}
helm.sh/chart: {{ include "smart-scheduler.chart" . }}
{{ include "smart-scheduler.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/part-of: smart-scheduler
{{- end }}

{{/*
Selector labels
*/}}
{{- define "smart-scheduler.selectorLabels" -}}
app.kubernetes.io/name: {{ include "smart-scheduler.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "smart-scheduler.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "smart-scheduler.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Create the namespace name
*/}}
{{- define "smart-scheduler.namespace" -}}
{{- default .Release.Namespace .Values.namespace }}
{{- end }}

{{/*
Create webhook service name
*/}}
{{- define "smart-scheduler.webhookServiceName" -}}
{{- printf "%s-webhook-service" (include "smart-scheduler.fullname" .) }}
{{- end }}

{{/*
Create webhook configuration name
*/}}
{{- define "smart-scheduler.webhookConfigName" -}}
{{- printf "%s-mutating-webhook-configuration" (include "smart-scheduler.fullname" .) }}
{{- end }}

{{/*
Create certificate issuer name
*/}}
{{- define "smart-scheduler.issuerName" -}}
{{- if .Values.certificates.certManager.issuer.existing }}
{{- .Values.certificates.certManager.issuer.existing }}
{{- else }}
{{- printf "%s-selfsigned-issuer" (include "smart-scheduler.fullname" .) }}
{{- end }}
{{- end }}

{{/*
Create certificate name
*/}}
{{- define "smart-scheduler.certificateName" -}}
{{- printf "%s-serving-cert" (include "smart-scheduler.fullname" .) }}
{{- end }}

{{/*
Create webhook secret name
*/}}
{{- define "smart-scheduler.webhookSecretName" -}}
{{- printf "%s-webhook-server-cert" (include "smart-scheduler.fullname" .) }}
{{- end }}

{{/*
Create monitoring labels
*/}}
{{- define "smart-scheduler.monitoringLabels" -}}
{{- if .Values.monitoring.serviceMonitor.labels }}
{{- toYaml .Values.monitoring.serviceMonitor.labels }}
{{- end }}
{{- end }}

{{/*
Create prometheus rule labels
*/}}
{{- define "smart-scheduler.prometheusRuleLabels" -}}
{{- if .Values.monitoring.prometheusRule.labels }}
{{- toYaml .Values.monitoring.prometheusRule.labels }}
{{- end }}
{{- end }}

{{/*
Webhook failure policy
*/}}
{{- define "smart-scheduler.webhookFailurePolicy" -}}
{{- if .Values.development.enabled }}
Ignore
{{- else }}
{{- .Values.webhook.failurePolicy }}
{{- end }}
{{- end }} 