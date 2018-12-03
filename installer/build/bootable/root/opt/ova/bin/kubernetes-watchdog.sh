#!/usr/bin/bash
set -euf -o pipefail

DCUI_SOCKET="UNIX:/var/run/dcui.sock"

function log_to_dcui() {
  local msg=$1
  echo -n "$msg" | socat $DCUI_SOCKET -
}

function check_api_endpoint() {
  local interval=$1
  local max_retries=$2
  local is_ready=$3
  local retries=0
  local JSONPATH='{range .items[*]}{@.metadata.name}:{range @.status.conditions[*]}{@.type}={@.status};{end}{end}'
  while [ $retries -lt "$max_retries" ]; do
    if kubectl --kubeconfig=/etc/kubernetes/admin.conf get nodes -o jsonpath="$JSONPATH" | grep "Ready=${is_ready}"; then
      return 0
    fi
    (( retries=retries+1 ))
    sleep "$interval"
  done
  return 1
}

# If kubelet is running and ready, report that is running and quit
if check_api_endpoint 2 5 True; then
  log_to_dcui "[RUNNING](fg-green)"
  exit 0
fi

# If kubelet is running and ready, report that is running and quit
if check_api_endpoint 2 5 False; then
  log_to_dcui "[NODE NOT READY](fg-red)"
  exit 0
fi
