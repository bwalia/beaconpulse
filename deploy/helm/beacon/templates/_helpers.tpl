{{/*
Common labels applied to every resource. Kept minimal and stable so selectors
never drift.
*/}}
{{- define "beacon.labels" -}}
app.kubernetes.io/part-of: beacon
app.kubernetes.io/managed-by: {{ .Release.Service }}
beacon.env: {{ .Values.env | quote }}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" }}
{{- end -}}

{{/*
Per-component selector labels. Usage: {{ include "beacon.selector" "api" }}
*/}}
{{- define "beacon.selector" -}}
app: beacon-{{ . }}
{{- end -}}

{{/*
Kubernetes resource name for a Beacon component: beacon-<component>.
Every Deployment, Service, and in-cluster DNS reference goes through this helper,
so a rename can never leave a dangling reference behind. The namespace is shared
with other products (diytaxreturn-*), so the beacon- prefix is what makes our
workloads identifiable at a glance in `kubectl get pods`.
Usage: {{ include "beacon.name" "api" }} -> beacon-api
*/}}
{{- define "beacon.name" -}}
beacon-{{ . }}
{{- end -}}

{{/*
Fully-qualified image reference: <registry>/<namespace>/<name>:<tag>.
The registry host lives in exactly one value, so the three images can never point
at different registries.
Usage: {{ include "beacon.image" (dict "root" . "name" "api") }}
*/}}
{{- define "beacon.image" -}}
{{- $img := .root.Values.image -}}
{{ $img.registry }}/{{ $img.namespace }}/{{ .name }}:{{ $img.tag }}
{{- end -}}

{{/*
The gateway base URL for this environment (always HTTPS behind cert-manager).
*/}}
{{- define "beacon.baseURL" -}}
https://{{ .Values.host }}
{{- end -}}

{{/*
Full backend (api + worker) environment. Secrets are pulled from beacon-secrets
via secretKeyRef; POSTGRES_PASSWORD is defined first so BEACON_DB_DSN can expand
$(POSTGRES_PASSWORD). Non-secret config is rendered inline. Usage:
  {{- include "beacon.backendEnv" . | nindent 12 }}
*/}}
{{- define "beacon.backendEnv" -}}
- name: POSTGRES_PASSWORD
  valueFrom:
    secretKeyRef:
      name: beacon-secrets
      key: POSTGRES_PASSWORD
- name: BEACON_DB_DSN
  value: "postgres://{{ .Values.postgres.user }}:$(POSTGRES_PASSWORD)@{{ include "beacon.name" "postgres" }}:5432/{{ .Values.postgres.database }}?sslmode=disable"
- name: BEACON_JWT_ACCESS_SECRET
  valueFrom:
    secretKeyRef:
      name: beacon-secrets
      key: BEACON_JWT_ACCESS_SECRET
- name: BEACON_JWT_REFRESH_SECRET
  valueFrom:
    secretKeyRef:
      name: beacon-secrets
      key: BEACON_JWT_REFRESH_SECRET
- name: BEACON_ENCRYPTION_KEY
  valueFrom:
    secretKeyRef:
      name: beacon-secrets
      key: BEACON_ENCRYPTION_KEY
- name: BEACON_WEBHOOK_TOKEN
  valueFrom:
    secretKeyRef:
      name: beacon-secrets
      key: BEACON_WEBHOOK_TOKEN
- name: BEACON_ENV
  value: {{ .Values.app.beaconEnv | quote }}
- name: BEACON_LOG_LEVEL
  value: {{ .Values.app.logLevel | quote }}
- name: BEACON_LOG_FORMAT
  value: {{ .Values.app.logFormat | quote }}
- name: BEACON_REDIS_ADDR
  value: "{{ include "beacon.name" "redis" }}:6379"
- name: BEACON_CORS_ORIGINS
  value: {{ include "beacon.baseURL" . | quote }}
- name: BEACON_DASHBOARD_URL
  value: {{ include "beacon.baseURL" . | quote }}
- name: BEACON_PROM_SCRAPE_FILE
  value: "/etc/prometheus/generated/scrape_monitors.yml"
- name: BEACON_PROM_RULES_FILE
  value: "/etc/prometheus/generated/rules_monitors.yml"
- name: BEACON_PROM_RELOAD_URL
  value: "http://{{ include "beacon.name" "prometheus" }}:9090/-/reload"
- name: BEACON_PROM_QUERY_URL
  value: "http://{{ include "beacon.name" "prometheus" }}:9090"
- name: BEACON_BLACKBOX_CONFIG_FILE
  value: "/etc/blackbox/blackbox.yml"
- name: BEACON_BLACKBOX_RELOAD_URL
  value: "http://{{ include "beacon.name" "blackbox" }}:9115/-/reload"
- name: BEACON_BLACKBOX_ADDR
  value: "{{ include "beacon.name" "blackbox" }}:9115"
- name: BEACON_DNS_RESOLVER
  value: {{ .Values.app.dnsResolver | quote }}
- name: BEACON_AI_ENABLED
  value: {{ .Values.ai.enabled | quote }}
{{- if .Values.ai.enabled }}
- name: BEACON_AI_BASE_URL
  value: {{ .Values.ai.baseURL | quote }}
- name: BEACON_AI_MODEL
  value: {{ .Values.ai.model | quote }}
- name: BEACON_AI_TIMEOUT
  value: {{ .Values.ai.timeout | quote }}
- name: BEACON_AI_API_KEY
  valueFrom:
    secretKeyRef:
      name: beacon-secrets
      key: BEACON_AI_API_KEY
      # Optional so enabling AI never blocks pod start if the Vault key isn't
      # populated yet — enrichment just degrades to "no analysis" (non-fatal).
      optional: true
{{- end }}
{{- end -}}

{{/*
imagePullSecrets block for pods that pull Beacon images from the private
SpectonCR registry. Renders nothing when no pullSecret is configured.
*/}}
{{- define "beacon.imagePullSecrets" -}}
{{- if .Values.image.pullSecret }}
imagePullSecrets:
  - name: {{ .Values.image.pullSecret }}
{{- end }}
{{- end -}}

{{/*
Shared volumes + mounts for control-plane surfaces (api/worker).
*/}}
{{- define "beacon.backendVolumeMounts" -}}
- name: prom-generated
  mountPath: /etc/prometheus/generated
- name: blackbox-config
  mountPath: /etc/blackbox
{{- end -}}

{{- define "beacon.backendVolumes" -}}
- name: prom-generated
  persistentVolumeClaim:
    claimName: beacon-prom-generated
- name: blackbox-config
  persistentVolumeClaim:
    claimName: beacon-blackbox-config
{{- end -}}
