#!/bin/bash

# Test if bao command is available
if ! command -v bao &> /dev/null; then
    echo "Error: bao command not found. Please install OpenBao CLI."
    exit 1
fi

# Test bao connectivity and authentication
echo "Testing OpenBao connectivity..."
if ! bao status > /dev/null 2>&1; then
    echo "Error: Unable to connect to OpenBao or authentication failed."
    echo "Please ensure OpenBao is running and you are authenticated."
    exit 1
fi

echo "OpenBao is accessible."

# Retrieve PKI tokens from OpenBao KV store
echo "Retrieving PKI tokens from OpenBao..."
export PKIISSUETOKENDEV=$(bao kv get -namespace=railway -field=PKIADMINTOKENDEV secret/tokens)
export PKIADMINTOKENDEV=$(bao kv get -namespace=railway -field=PKIISSUETOKENDEV secret/tokens)
export PKIISSUETOKENSTAGING=$(bao kv get -namespace=railway -field=PKIADMINTOKENSTAGING secret/tokens)
export PKIADMINTOKENSTAGING=$(bao kv get -namespace=railway -field=PKIISSUETOKENSTAGING secret/tokens)
export PKIISSUETOKENPROD=$(bao kv get -namespace=railway -field=PKIADMINTOKENPROD secret/tokens)
export PKIADMINTOKENPROD=$(bao kv get -namespace=railway -field=PKIISSUETOKENPROD secret/tokens)
echo "PKI tokens successfully exported to environment variables."

echo "Retrieving RabbitMQ credentials from OpenBao..."
export RABBITMQ_DEFAULT_USER=$(bao kv get -namespace=railway -field=RABBITMQ_DEFAULT_USER secret/tokens)
export RABBITMQ_DEFAULT_PASS=$(bao kv get -namespace=railway -field=RABBITMQ_DEFAULT_PASS secret/tokens)
export RABBITMQ_TRAIN_USER=$(bao kv get -namespace=railway -field=RABBITMQ_TRAIN_USER secret/tokens)
export RABBITMQ_TRAIN_PASS=$(bao kv get -namespace=railway -field=RABBITMQ_TRAIN_PASS secret/tokens)
echo "RabbitMQ credentials successfully exported to environment variables."

