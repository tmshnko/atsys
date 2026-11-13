#!/bin/bash

set -euo pipefail

for f in sql/*.sql; do
    echo "Executing $f"
    clickhouse-client \
        --host clickhouse \
        --port $CH_PORT \
        --user $CLICKHOUSE_USER \
        --password $CLICKHOUSE_PASSWORD \
        --multiquery < $f
    echo "Completed $f"
done

echo "All Clickhouse scripts completed successfully"