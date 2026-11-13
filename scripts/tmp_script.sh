#!/bin/bash
DIR_NAME=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)

source "${DIR_NAME}/../.env"
export GRAFANA_CH_USER DB_NAME GRAFANA_CH_PASSWORD CH_HOST

envsubst < ./sql/templates/002_add_grafana_user_temp.sql > ./sql/sys_metrics/002_add_grafana_user.sql

