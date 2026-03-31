#!/bin/sh
set -e

# set target based on env var
TARGET=${EXPORTER_TARGET:-"exporter-staging:8080"}

echo "Configuring Prometheus to scrape: $TARGET"

sed "s/\${EXPORTER_TARGET}/$TARGET/g" /etc/prometheus/prometheus.yaml.template > /etc/prometheus/prometheus.yaml

# prometheus entrypoint
exec /bin/prometheus \
    --config.file=/etc/prometheus/prometheus.yaml \
    --storage.tsdb.path=/prometheus \
    --storage.tsdb.retention.time=1h \
    --web.console.libraries=/usr/share/prometheus/console_libraries \
    --web.console.templates=/usr/share/prometheus/consoles