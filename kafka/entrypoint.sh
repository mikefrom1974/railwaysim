#!/bin/bash
set -e

echo "Starting Go PKI Enroller..."
/usr/local/bin/kafka-enroller

# Convert PKCS#1 (RSA) to PKCS#8
openssl pkcs8 -topk8 -inform PEM -outform PEM -nocrypt -in /etc/kafka/certs/key.pem -out /etc/kafka/certs/key.pkcs8.pem

# Overwrite the original
mv /etc/kafka/certs/key.pkcs8.pem /etc/kafka/certs/key.pem

# cat the keys into a combined keystore
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
export KAFKA_SSL_CLIENT_AUTH="requested" # set to required for full mTLS
# Tell Kafka it's OK to have just 1 replica for this lab
export KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR=1
export KAFKA_TRANSACTION_STATE_LOG_REPLICATION_FACTOR=1
export KAFKA_TRANSACTION_STATE_LOG_MIN_ISR=1

export PATH=$PATH:/opt/kafka/bin

# Create a client mTLS config (needed for CLI tools)
cat <<EOF > /tmp/client-ssl.properties
security.protocol=SSL
ssl.truststore.type=PEM
ssl.truststore.location=/etc/kafka/certs/ca.pem
ssl.keystore.type=PEM
ssl.keystore.location=/etc/kafka/certs/keystore.pem
ssl.endpoint.identification.algorithm=
EOF

# Run a loop to create the topic when kafka is ready
#  Note that in prod k8s we'd use the Strimzi Operator to manage topics declaratively
echo "Creating train-telemetry topic, waiting for Kafka to be ready..."
(
    # wait for port to open
    while ! nc -z localhost 9092; do
        sleep 1
    done

    echo "Create topic: Kafka is up, creating 'train-telemetry' topic"
    kafka-topics.sh --create \
        --if-not-exists \
        --topic train-telemetry \
        --bootstrap-server localhost:9092 \
        --command-config /tmp/client-ssl.properties \
        --replication-factor 1 \
        --partitions 3
) &

# 4. Hand off to the official Apache script
echo "Kafka environment prepped. Starting Kafka..."
exec /etc/kafka/docker/run