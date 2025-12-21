{{/*
Expand the name of the chart.
*/}}
{{- define "predictor.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "predictor.fullname" -}}
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
{{- define "predictor.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "predictor.labels" -}}
helm.sh/chart: {{ include "predictor.chart" . }}
{{ include "predictor.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "predictor.selectorLabels" -}}
app.kubernetes.io/name: {{ include "predictor.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Resource Agent labels
*/}}
{{- define "predictor.agent.labels" -}}
{{ include "predictor.labels" . }}
app.kubernetes.io/component: resource-agent
{{- end }}

{{/*
Resource Agent selector labels
*/}}
{{- define "predictor.agent.selectorLabels" -}}
{{ include "predictor.selectorLabels" . }}
app.kubernetes.io/component: resource-agent
{{- end }}

{{/*
Recommendation API labels
*/}}
{{- define "predictor.api.labels" -}}
{{ include "predictor.labels" . }}
app.kubernetes.io/component: recommendation-api
{{- end }}

{{/*
Recommendation API selector labels
*/}}
{{- define "predictor.api.selectorLabels" -}}
{{ include "predictor.selectorLabels" . }}
app.kubernetes.io/component: recommendation-api
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "predictor.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "predictor.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Resource Agent image
*/}}
{{- define "predictor.agent.image" -}}
{{- $registry := .Values.global.imageRegistry | default "" -}}
{{- $repository := .Values.resourceAgent.image.repository -}}
{{- $tag := .Values.resourceAgent.image.tag | default .Chart.AppVersion -}}
{{- if $registry }}
{{- printf "%s/%s:%s" $registry $repository $tag }}
{{- else }}
{{- printf "%s:%s" $repository $tag }}
{{- end }}
{{- end }}

{{/*
Recommendation API image
*/}}
{{- define "predictor.api.image" -}}
{{- $registry := .Values.global.imageRegistry | default "" -}}
{{- $repository := .Values.recommendationApi.image.repository -}}
{{- $tag := .Values.recommendationApi.image.tag | default .Chart.AppVersion -}}
{{- if $registry }}
{{- printf "%s/%s:%s" $registry $repository $tag }}
{{- else }}
{{- printf "%s:%s" $repository $tag }}
{{- end }}
{{- end }}

{{/*
Database connection string
*/}}
{{- define "predictor.databaseUrl" -}}
{{- if .Values.database.external }}
{{- .Values.database.connectionString }}
{{- else }}
{{- printf "postgres://%s:$(DB_PASSWORD)@%s:%d/%s?sslmode=disable" .Values.database.user .Values.database.host (int .Values.database.port) .Values.database.name }}
{{- end }}
{{- end }}
