#!/bin/bash
set -e

echo "Starting Go PKI Enroller..."
/usr/local/bin/kafka-enroller

# Convert PKCS#1 (RSA) to PKCS#8
openssl pkcs8 -topk8 -inform PEM -outform PEM -nocrypt -in /etc/kafka/certs/key.pem -out /etc/kafka/certs/key.pkcs8.pem

# Overwrite the original
mv /etc/kafka/certs/key.pkcs8.pem /etc/kafka/certs/key.pem

# cat the keys directly to the environment variables
cat /etc/kafka/certs/key.pem /etc/kafka/certs/cert.pem > /etc/kafka/certs/keystore.pem

# --- KRaft Storage Formatting ---
# Kafka needs a Cluster ID. We set this in docker-compose to specify different for prod/staging
echo "Formatting storage with Cluster ID: $CLUSTER_ID"
/opt/kafka/bin/kafka-storage.sh format -t $CLUSTER_ID -c /etc/kafka/docker/server.properties --ignore-formatted


# Set config via environment variables for IaC
export KAFKA_NODE_ID=1
export KAFKA_PROCESS_ROLES=broker,controller
export KAFKA_CLUSTER_ID=$CLUSTER_ID
export KAFKA_LISTENERS="CLIENT://:9092,CONTROLLER://:9093"
export KAFKA_LISTENER_SECURITY_PROTOCOL_MAP="CLIENT:SSL,CONTROLLER:SSL"
export KAFKA_INTER_BROKER_LISTENER_NAME="CLIENT"
export KAFKA_CONTROLLER_LISTENER_NAMES="CONTROLLER"
export KAFKA_SSL_KEYSTORE_TYPE="PEM"
export KAFKA_SSL_KEYSTORE_LOCATION="/etc/kafka/certs/keystore.pem"
export KAFKA_SSL_TRUSTSTORE_TYPE="PEM"
export KAFKA_SSL_TRUSTSTORE_LOCATION="/etc/kafka/certs/ca.pem"
export KAFKA_SSL_CLIENT_AUTH="required"


echo "Kafka environment prepped. Starting Kafka..."

# 4. Hand off to the official Apache script
exec /etc/kafka/docker/run