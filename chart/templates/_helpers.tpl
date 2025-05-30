{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "opengauss-operator.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.opengaussOperator.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.opengaussOperator.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create name for leader election role.
*/}}
{{- define "opengauss-operator.leaderElectionRole" -}}
{{- printf "%s-leader-election-role" .Release.Name | trunc 63 | trimSuffix "-" }}
{{- end }}