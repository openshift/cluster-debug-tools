#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

# Ingest logs collected in a must-gather into loki via promtail

MUST_GATHER_PATH="${1-}"
if [[ -z "${MUST_GATHER_PATH}" ]]; then
  >&2 echo "Usage: $0 /path/to/untarred/must-gather/root"
  exit 1
fi

if [[ "${LOKI_ADDR}" == "" ]]; then
  >&2 echo "LOKI_ADDR is not set (e.g. LOKI_ADDR=http://localhost:3100)"
  exit 1
fi


function ingest-pod-log() {
  local log_file="${1}"

  local filename
  filename="$(basename "${log_file}")"

  local current_dir
  current_dir="$(dirname "${log_file}")" # 'logs' dir (ignore)
  current_dir="$(dirname "${current_dir}")" # first container dir (ignore)
  current_dir="$(dirname "${current_dir}")" # second container dir

  local container
  container="$(basename "${current_dir}")"

  current_dir="$(dirname "${current_dir}")" # pod dir

  local pod
  pod="$(basename "${current_dir}")"

  current_dir="$(dirname "${current_dir}")" # pods dir
  current_dir="$(dirname "${current_dir}")" # namespace dir

  local namespace
  namespace="$(basename "${current_dir}")"

  local config_file
  config_file="$(mktemp -p "${CONFIG_DIR}")"

  # TODO Discover the service account name from the pod yaml

  # Generate configuration to tmp file.
  # Containing dir will be cleaned up.
  cat <<EOF > "${config_file}"
# Server isn't required
server:
  disable: true

client:
  url: ${LOKI_ADDR}/loki/api/v1/push

scrape_configs:
- job_name: must-gather-pod
  pipeline_stages:
  - regex:
      expression: ^(?s)(?P<time>\\S+?) (?P<content>.*)$
  - timestamp:
      source: time
      format: RFC3339Nano
  - output:
      source: content

  static_configs:
  - labels:
      namespace: ${namespace}
      pod_name: ${pod}
      pod_container_name: ${container}
      filename: ${filename}
EOF
  echo "Ingesting pod log: ${log_file}"
  cat "${log_file}" | promtail --stdin --config.file="${config_file}" &
  echo ""
}


function ingest-host-service-log() {
  local log_file="${1}"
  # TODO
}


function ingest-audit-log() {
  local log_file="${1}"

  local filename
  filename="$(basename "${log_file}")"

  local node
  node="$(echo "${filename}" | sed -e 's+\(.*\)-audit.*+\1+')"

  local apiserver_dir
  apiserver_dir="$(dirname "${log_file}")"
  local apiserver
  apiserver="$(basename "${apiserver_dir}")"

  local config_file
  config_file="$(mktemp -p "${CONFIG_DIR}")"

  # Generate configuration to tmp file.
  # Containing dir will be cleaned up.
  cat <<EOF > "${config_file}"
# Server isn't required
server:
  disable: true

client:
  url: ${LOKI_ADDR}/loki/api/v1/push

scrape_configs:
- job_name: must-gather-audit
  pipeline_stages:
  - json:
      expressions:
        stage:
        requestURI:
        verb:
        user:
        sourceIPs:
        objectRef:
        responseStatus:
        requestReceivedTimestamp:
        stageTimestamp:
        annotations:
  - json:
      expressions:
        username:
      source: user
  - json:
      expressions:
        resource:
        namespace:
        name:
      source: objectRef
  - json:
      expressions:
        responseCode: code
      source: responseStatus
  - json:
      expressions:
        decision: '"authorization.k8s.io/decision"'
        reason: '"authorization.k8s.io/reason"'
      source: annotations
  - regex:
      expression: ^\[\"(?P<sourceIP>[\d\.]+)\"\]
      source: sourceIPs
  - labels:
      stage:
      requestURI:
      verb:
      username:
      sourceIP:
      resource:
      namespace:
      name:
      responseCode:
      decision:
  - timestamp:
      source: stageTimestamp
      format: RFC3339Nano

  static_configs:
  - labels:
      node: ${node}
      apiserver: ${apiserver}
      filename: ${filename}
EOF
  echo "Ingesting audit log: ${log_file}"
  # Loki requires that events have be received in sorted order.  Sort
  # by the same stageTimestamp field that will be scraped as the
  # timestamp.
  gunzip -c "${log_file}" |\
    sed -e 's+\(.*stageTimestamp":"\([^"]*\)".*\)+\2 \1+' |\
    sort |\
    sed -e 's+[^{]*\(.*\)+\1+' |\
    promtail --stdin --config.file="${config_file}"
  echo ""
}


CONFIG_DIR="$(mktemp -d /tmp/loki-ingest-XXX)"
# deletes the temp directory
function cleanup {
  rm -rf "$CONFIG_DIR"
  echo "Deleted temp config directory $CONFIG_DIR"
}
# register the cleanup function to be called on the EXIT signal
trap cleanup EXIT

LOG_FILES="$(find "${MUST_GATHER_PATH}" -name '*.log')"
for log_file in ${LOG_FILES[@]}; do
  if [[ "${log_file}" == *"/namespaces/"* ]]; then
    ingest-pod-log "${log_file}"
  elif [[ "${log_file}" == *"/host_service_logs/"* ]]; then
    ingest-host-service-log "${log_file}"
  fi
done

AUDIT_LOG_FILES="$(find "${MUST_GATHER_PATH}" -name '*.log.gz')"
for log_file in ${AUDIT_LOG_FILES[@]}; do
  # TODO audit logs across files
  ingest-audit-log "${log_file}"
done
