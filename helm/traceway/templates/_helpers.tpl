{{- define "traceway.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "traceway.fullname" -}}
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

{{- define "traceway.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "traceway.labels" -}}
helm.sh/chart: {{ include "traceway.chart" . }}
{{ include "traceway.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{- define "traceway.selectorLabels" -}}
app.kubernetes.io/name: {{ include "traceway.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{- define "traceway.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "traceway.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/* Release images are tagged v<version>[-<variant>]; the full variant has no suffix */}}
{{- define "traceway.imageTag" -}}
{{- if .Values.image.tag -}}
{{ .Values.image.tag }}
{{- else -}}
{{- $version := printf "v%s" (trimPrefix "v" .Chart.AppVersion) -}}
{{- if eq .Values.variant "full" -}}
{{ $version }}
{{- else -}}
{{ $version }}-{{ .Values.variant }}
{{- end -}}
{{- end -}}
{{- end }}

{{/* Variants that keep their databases as local files under /data */}}
{{- define "traceway.embeddedDB" -}}
{{- if has .Values.variant (list "sqlite" "duckdb") -}}true{{- end -}}
{{- end }}

{{/* Name of the Secret to mount — either an existing one or the one we create */}}
{{- define "traceway.secretName" -}}
{{- if .Values.existingSecret -}}
{{ .Values.existingSecret }}
{{- else -}}
{{ include "traceway.fullname" . }}
{{- end -}}
{{- end }}

{{/* Name of the PVC to mount */}}
{{- define "traceway.pvcName" -}}
{{- if .Values.persistence.existingClaim -}}
{{ .Values.persistence.existingClaim }}
{{- else -}}
{{ include "traceway.fullname" . }}
{{- end -}}
{{- end }}
