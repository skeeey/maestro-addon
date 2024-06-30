{{/*
Sign Kafka admin client certs
*/}}
{{- define "kafka.client-certs" -}}
{{- $ca := dict "Cert" "" "Key" "" -}}
{{- $caCertData := (lookup "v1" "Secret" .Values.messageQueue.amqStreams.namespace (printf "%s-clients-ca-cert" .Values.messageQueue.amqStreams.name)).data -}}
{{- range $key, $value := $caCertData }}
    {{- if eq $key "ca.crt" }}
        {{- $_ := set $ca "Cert" ($value | toString ) -}}
    {{- end }}
{{- end }}
{{- $caKeyData := (lookup "v1" "Secret" .Values.messageQueue.amqStreams.namespace (printf "%s-clients-ca" .Values.messageQueue.amqStreams.name)).data -}}
{{- range $key, $value := $caKeyData }}
    {{- if eq $key "ca.key" }}
        {{- $_ := set $ca "Key" ($value | toString) -}}
    {{- end }}
{{- end }}
{{- $sprigCA := buildCustomCert $ca.Cert $ca.Key }}
{{- $clientCert := genSignedCert "maestro-kafka-admin" nil (list "maestro") 365 $sprigCA -}}
client.crt: {{ $clientCert.Cert | toString | b64enc }}
client.key: {{ $clientCert.Key | toString | b64enc }}
{{- end -}}
