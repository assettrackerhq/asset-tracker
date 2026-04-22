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
Proxy image helper - rewrites image refs for air gap (local registry) or
through the Replicated proxy registry, otherwise returns the image unchanged.
Precedence:
  1. global.localRegistry.host + namespace set → air gap: LocalRegistryHost/LocalRegistryNamespace/<last-segment>:<tag>
  2. global.proxy.enabled + domain + appSlug set → proxy.replicated.com/proxy/<appSlug>/<image>
  3. otherwise → image as-is
Usage: {{ include "asset-tracker.proxyImage" (dict "root" . "image" "docker.io/library/busybox:1.37") }}
*/}}
{{- define "asset-tracker.proxyImage" -}}
{{- $global := .root.Values.global | default dict -}}
{{- $local := $global.localRegistry | default dict -}}
{{- $proxy := $global.proxy | default dict -}}
{{- if and (index $local "host") (index $local "namespace") -}}
{{- $last := .image | splitList "/" | last -}}
{{ $local.host }}/{{ $local.namespace }}/{{ $last }}
{{- else if and (index $proxy "enabled") (index $proxy "domain") (index $proxy "appSlug") -}}
{{ $proxy.domain }}/proxy/{{ $proxy.appSlug }}/{{ .image }}
{{- else -}}
{{ .image }}
{{- end -}}
{{- end -}}

{{/*
Image pull secrets - includes enterprise-pull-secret when dockerconfigjson is
injected by Replicated, plus any customer-provided pull secrets.
*/}}
{{- define "asset-tracker.imagePullSecrets" -}}
  {{- $pullSecrets := list }}
  {{- with ((.Values.global).imagePullSecrets) -}}
    {{- range . -}}
      {{- if kindIs "map" . -}}
        {{- $pullSecrets = append $pullSecrets .name -}}
      {{- else -}}
        {{- $pullSecrets = append $pullSecrets . -}}
      {{- end }}
    {{- end -}}
  {{- end -}}
  {{- with .Values.images -}}
    {{- range .pullSecrets -}}
      {{- if kindIs "map" . -}}
        {{- $pullSecrets = append $pullSecrets .name -}}
      {{- else -}}
        {{- $pullSecrets = append $pullSecrets . -}}
      {{- end -}}
    {{- end -}}
  {{- end -}}
  {{- if hasKey (default dict ((.Values.global).replicated)) "dockerconfigjson" }}
    {{- $pullSecrets = append $pullSecrets "enterprise-pull-secret" -}}
  {{- end -}}
  {{- if (not (empty $pullSecrets)) }}
imagePullSecrets:
    {{- range $pullSecrets | uniq }}
  - name: {{ . }}
    {{- end }}
  {{- end }}
{{- end -}}

{{/*
Image pull secret names as a comma-separated string (for passing as env var).
*/}}
{{- define "asset-tracker.imagePullSecretNames" -}}
  {{- $pullSecrets := list }}
  {{- with ((.Values.global).imagePullSecrets) -}}
    {{- range . -}}
      {{- if kindIs "map" . -}}
        {{- $pullSecrets = append $pullSecrets .name -}}
      {{- else -}}
        {{- $pullSecrets = append $pullSecrets . -}}
      {{- end }}
    {{- end -}}
  {{- end -}}
  {{- with .Values.images -}}
    {{- range .pullSecrets -}}
      {{- if kindIs "map" . -}}
        {{- $pullSecrets = append $pullSecrets .name -}}
      {{- else -}}
        {{- $pullSecrets = append $pullSecrets . -}}
      {{- end -}}
    {{- end -}}
  {{- end -}}
  {{- if hasKey (default dict ((.Values.global).replicated)) "dockerconfigjson" }}
    {{- $pullSecrets = append $pullSecrets "enterprise-pull-secret" -}}
  {{- end -}}
  {{- join "," ($pullSecrets | uniq) -}}
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
