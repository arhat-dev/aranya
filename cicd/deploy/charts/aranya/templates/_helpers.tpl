{{/* vim: set filetype=mustache: */}}
{{/*
Expand the name of the chart.
*/}}
{{- define "aranya.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "aranya.fullname" -}}
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
{{- define "aranya.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "aranya.labels" -}}
helm.sh/chart: {{ include "aranya.chart" $ }}
{{ include "aranya.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "aranya.selectorLabels" -}}
app.kubernetes.io/name: {{ include "aranya.name" $ }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "aranya.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "aranya.fullname" $) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{- define "aranya.metricsPort" -}}
{{- index (split ":" (default ":0" .Values.config.aranya.metrics.endpoint)) "_1" -}}
{{- end }}

{{- define "aranya.pprofPort" -}}
{{- index (split ":" (default ":0" .Values.config.aranya.pprof.listen)) "_1" -}}
{{- end }}

{{- define "sysNamespace" -}}
{{- .Values.sysNamespace | default .Release.Namespace | toString -}}
{{- end }}

{{- define "tenantNamespace" -}}
{{- .Values.tenantNamespace | default (include "sysNamespace" $) | toString -}}
{{- end }}

{{- define "lock.endpoints" -}}
{{- if .Values.config.aranya.leaderElection.lock.type | toString | contains "endpoints" -}}
"yes"
{{- end -}}
{{- end }}

{{- define "lock.configmaps" -}}
{{- if .Values.config.aranya.leaderElection.lock.type | toString | contains "configmaps" -}}
"yes"
{{- end -}}
{{- end }}

{{- define "lock.leases" -}}
{{- if .Values.config.aranya.leaderElection.lock.type | toString | contains "leases" -}}
"yes"
{{- end -}}
{{- end }}

{{- define "manageTenantPodRoles" -}}
{{- if ne (len .Values.config.aranya.managed.podRoles) 0 -}}
"yes"
{{- end -}}
{{- end }}

{{- define "manageVirtualPodRoles" -}}
{{- if ne (len .Values.config.aranya.managed.virtualPodRoles) 0 -}}
"yes"
{{- end -}}
{{- end }}

{{- define "manageAbbotService" -}}
{{- if ne (len .Values.config.virtualnode.network.abbotService.name) 0 -}}
"yes"
{{- end -}}
{{- end }}

{{- define "manageNetworkService" -}}
{{- if ne (len .Values.config.virtualnode.network.networkService.name) 0 -}}
"yes"
{{- end -}}
{{- end }}
