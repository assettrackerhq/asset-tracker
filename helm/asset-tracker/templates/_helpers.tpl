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
{{- if .Values.postgresql.enabled -}}
{{ .Release.Name }}-postgresql
{{- else -}}
{{ .Values.externalDatabase.host }}
{{- end -}}
{{- end }}

{{/*
PostgreSQL port
*/}}
{{- define "asset-tracker.postgresPort" -}}
{{- if .Values.postgresql.enabled -}}
5432
{{- else -}}
{{ .Values.externalDatabase.port }}
{{- end -}}
{{- end }}

{{/*
Proxy image helper - rewrites image refs through the Replicated proxy registry
Usage: {{ include "asset-tracker.proxyImage" (dict "root" . "image" "docker.io/library/busybox:1.37") }}
*/}}
{{- define "asset-tracker.proxyImage" -}}
{{- $global := .root.Values.global | default dict -}}
{{- $proxy := $global.proxy | default dict -}}
{{- if and (index $proxy "enabled") (index $proxy "domain") (index $proxy "appSlug") -}}
{{ $proxy.domain }}/proxy/{{ $proxy.appSlug }}/{{ .image }}
{{- else -}}
{{ .image }}
{{- end -}}
{{- end -}}

{{/*
Image pull secrets - adds enterprise-pull-secret when proxy is enabled
*/}}
{{- define "asset-tracker.imagePullSecrets" -}}
{{- $global := .Values.global | default dict -}}
{{- $proxy := $global.proxy | default dict -}}
{{- if index $proxy "enabled" }}
imagePullSecrets:
  - name: enterprise-pull-secret
{{- end -}}
{{- end -}}

{{/*
PostgreSQL connection URI
*/}}
{{- define "asset-tracker.databaseURL" -}}
{{- if .Values.postgresql.enabled -}}
postgres://{{ .Values.postgresql.auth.username }}:{{ .Values.postgresql.auth.password }}@{{ include "asset-tracker.postgresHost" . }}:{{ include "asset-tracker.postgresPort" . }}/{{ .Values.postgresql.auth.database }}?sslmode=disable
{{- else -}}
postgres://{{ .Values.externalDatabase.username }}:{{ .Values.externalDatabase.password }}@{{ include "asset-tracker.postgresHost" . }}:{{ include "asset-tracker.postgresPort" . }}/{{ .Values.externalDatabase.database }}?sslmode=disable
{{- end -}}
{{- end }}
