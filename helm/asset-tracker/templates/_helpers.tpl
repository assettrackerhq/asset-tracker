{{/*
Common labels
*/}}
{{- define "asset-tracker.labels" -}}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/instance: {{ .Release.Name }}
helm.sh/chart: {{ .Chart.Name }}-{{ .Chart.Version }}
{{- end }}

{{/*
Fullname helper
*/}}
{{- define "asset-tracker.fullname" -}}
{{ .Release.Name }}-{{ .Chart.Name }}
{{- end }}

{{/*
PostgreSQL host
*/}}
{{- define "asset-tracker.postgresHost" -}}
{{ .Release.Name }}-postgresql
{{- end }}

{{/*
PostgreSQL connection URI
*/}}
{{- define "asset-tracker.databaseURL" -}}
postgres://{{ .Values.postgresql.auth.username }}:{{ .Values.postgresql.auth.password }}@{{ include "asset-tracker.postgresHost" . }}:5432/{{ .Values.postgresql.auth.database }}?sslmode=disable
{{- end }}
