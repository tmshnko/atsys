#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ENV_FILE="$SCRIPT_DIR/../.env"

if [[ ! -s $ENV_FILE ]]; then
    echo "ENV not found or empty"
    exit 1
fi

source $ENV_FILE

if [[ -z ${DUCKDNS_SUBDOMAIN:-} || -z ${DUCKDNS_TOKEN:-} ]]; then
    echo "Duckdns variables do not exist"
    exit 1
fi

RESPONSE=$(curl -s "https://www.duckdns.org/update?domains=${DUCKDNS_SUBDOMAIN}&token=${DUCKDNS_TOKEN}&ip=")

if [[ $RESPONSE == "OK" ]]; then
    echo "Duckdns ip updated successfully"
else
    echo "Error while updating duckdns ip: $RESPONSE"
    exit 1
fi