# build command (run from the prometheus directory and match version to change log):
# docker build --platform linux/amd64 --build-arg VERSION=0.1.0 -t prometheus-server:0.1.0 .
FROM prom/prometheus:v3.10.0

# Copy config
COPY prometheus.yaml.template /etc/prometheus/prometheus.yaml.template
COPY entrypoint.sh /usr/local/bin/entrypoint.sh
USER root
RUN chmod +x /usr/local/bin/entrypoint.sh

EXPOSE 9090
USER 65534
ENTRYPOINT [ "/usr/local/bin/entrypoint.sh" ]