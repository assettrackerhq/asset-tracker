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
PostgreSQL connection URI
*/}}
{{- define "asset-tracker.databaseURL" -}}
postgres://{{ .Values.postgres.user }}:{{ .Values.postgres.password }}@{{ include "asset-tracker.fullname" . }}-postgres:5432/{{ .Values.postgres.database }}?sslmode=disable
{{- end }}
