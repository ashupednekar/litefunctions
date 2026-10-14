{{/*
Expand the name of the chart.
*/}}
{{- define "litefunctions.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "litefunctions.fullname" -}}
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
{{- define "litefunctions.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "litefunctions.labels" -}}
helm.sh/chart: {{ include "litefunctions.chart" . }}
{{ include "litefunctions.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "litefunctions.selectorLabels" -}}
app.kubernetes.io/name: {{ include "litefunctions.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Gateway API resource name used by Istio-managed gateway deployment/service.
*/}}
{{- define "litefunctions.gatewayName" -}}
litefunctions-gateway
{{- end }}

{{/* Hostnames shared by Gateway listeners, routes, and certificates. */}}
{{- define "litefunctions.gatewayHosts" -}}
{{- $portal := .Values.gatewayApi.hosts.portal | default (list "litefunctions.portal" (.Values.ui.domain | default "litefunctions.portal") | uniq) -}}
{{- $gitea := .Values.gatewayApi.hosts.gitea | default (list "litefunctions.gitea" "gitea.local") -}}
{{- $hosts := dict "portal" (list) "gitea" (list) "rustfs" (list) "rustfsConsole" (list) -}}
{{- $_ := set $hosts "portal" $portal -}}
{{- if .Values.gitea.enabled -}}
{{- $_ := set $hosts "gitea" $gitea -}}
{{- end -}}
{{- if .Values.rustfs.enabled -}}
{{- $_ := set $hosts "rustfs" (.Values.gatewayApi.hosts.rustfs | default (list)) -}}
{{- $_ := set $hosts "rustfsConsole" (.Values.gatewayApi.hosts.rustfsConsole | default (list)) -}}
{{- end -}}
{{- toYaml $hosts -}}
{{- end -}}
