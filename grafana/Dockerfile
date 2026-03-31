# build command (run from the bff directory and match version to change log):
# docker build --platform linux/amd64 --build-arg VERSION=0.1.0 -t grafana-server:0.1.0 .
FROM grafana/grafana:12.4.2

# Disable the initial login "change password" prompt for lab speed
ENV GF_SECURITY_ADMIN_PASSWORD=admin
ENV GF_AUTH_ANONYMOUS_ENABLED=true
ENV GF_AUTH_ANONYMOUS_ORG_ROLE=Admin

# Copy provisioning configs
COPY datasource.yaml /etc/grafana/provisioning/datasources/datasource.yaml
COPY dashboard.yaml /etc/grafana/provisioning/dashboards/dashboard.yaml

# Copy the actual dashboard JSON
COPY train-dashboard-staging.json /var/lib/grafana/dashboards/train-dashboard.json

EXPOSE 3000