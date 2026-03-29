#!/bin/bash
set -e

echo "Starting Go PKI Enroller..."
/usr/local/bin/kong-enroller

# Convert PKCS#1 (RSA) to PKCS#8
openssl pkcs8 -topk8 -inform PEM -outform PEM -nocrypt -in /etc/kong/certs/key.pem -out /etc/kong/certs/key.pkcs8.pem

# Overwrite the original
mv /etc/kong/certs/key.pkcs8.pem /etc/kong/certs/key.pem

# Save the CA to base64 in an environment variable (needed for DB-less Kong)
# Commented out because this was for the mtls_auth plugin
# export CA_PEM_BASE64=$(base64 -w 0 /etc/kong/certs/ca.pem)

# Render Kong declarative config
envsubst < /etc/kong/kong.yaml.template > /etc/kong/kong.yaml

# Set config via environment variables for IaC
export KONG_DATABASE=off
export KONG_DECLARATIVE_CONFIG=/etc/kong/kong.yaml
export KONG_PROXY_LISTEN="0.0.0.0:8443 ssl"
export KONG_ADMIN_LISTEN="0.0.0.0:8001"

# Enable community plugins (enterprise would use mtls-auth / kafka-log)
export KONG_PLUGINS="bundled,kong-kafka-log"

# mTLS Configuration
export KONG_SSL_CERT=/etc/kong/certs/cert.pem
export KONG_SSL_CERT_KEY=/etc/kong/certs/key.pem
export KONG_PROXY_SSL_CERT=/etc/kong/certs/cert.pem
export KONG_PROXY_SSL_CERT_KEY=/etc/kong/certs/key.pem
export KONG_NGINX_PROXY_SSL_VERIFY_CLIENT="on"
export KONG_NGINX_PROXY_SSL_CLIENT_CERTIFICATE=/etc/kong/certs/ca.pem
export KONG_NGINX_PROXY_SSL_VERIFY_DEPTH=2
export KONG_LUA_SSL_TRUSTED_CERTIFICATE=/etc/kong/certs/ca.pem

# turn off verification for our Kong > Ingester traffic since it would likely be a sidecar
export KONG_PROXY_SSL_VERIFY="off"

# Hand off to the official Kong entrypoint
echo "Starting Kong..."
exec /docker-entrypoint.sh kong docker-start

