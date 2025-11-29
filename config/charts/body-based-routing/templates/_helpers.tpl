
{{- define "body-based-routing.name" -}}
{{ .Chart.Name }}
{{- end }}

{{- define "body-based-routing.fullname" -}}
{{ .Release.Name }}-{{ .Chart.Name }}
{{- end }}

{{- define "body-based-routing.chart" -}}
{{ .Chart.Name }}-{{ .Chart.Version }}
{{- end }}