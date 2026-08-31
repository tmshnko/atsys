#!/bin/bash
DIR_NAME=$(cd $(dirname "${BASH_SOURCE[0]}") && pwd) 

source "${DIR_NAME}/.env"

envsubst < ./sql/sys_metrics/002_add_grafana_user.sql > output.tmp.sql

