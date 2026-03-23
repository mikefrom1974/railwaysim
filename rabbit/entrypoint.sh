#!/bin/bash
set -e

echo "Starting go PKI Enroller..."
/usr/local/bin/rabbit-enroller

# create dummy worker user for this sim only
make_user() {
    sleep 5
    echo "Waiting for MQ to wake up"
    rabbitmqctl wait /var/lib/rabbitmq/mnesia/rabbit@$HOSTNAME.pid --timeout 60

    echo "Resetting worker user"
    rabbitmqctl add_user "$RABBITMQ_TRAIN_USER" "$RABBITMQ_TRAIN_PASS" || rabbitmqctl change_password "$RABBITMQ_TRAIN_USER" "$RABBITMQ_TRAIN_PASS"
    rabbitmqctl set_permissions -p / "$RABBITMQ_TRAIN_USER" ".*" ".*" ".*"

    echo "Worker user is now ready"
}
make_user &

# start our rabbit MQ server as the original docker container would
echo "Starting RabbitMQ Server..."
exec docker-entrypoint.sh rabbitmq-server