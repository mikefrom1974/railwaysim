#!/bin/sh
set -e

# ensure redis user and pw are set
if [ -z "${REDIS_USER}" ]; then
    echo "Error: REDIS_USER environment variable is not set" >&2
    exit 1
fi
if [ -z "${REDIS_PASS}" ]; then
    echo "Error: REDIS_PASS environment variable is not set" >&2
    exit 1
fi

# ACL path
ACL_FILE="/usr/local/etc/redis/users.acl"

# Generate ACL
REDIS_HASH=$(echo -n "$REDIS_PASS" | sha256sum | cut -d ' ' -f 1)
unset REDIS_PASS
echo "user default off" > "$ACL_FILE"
echo "user ${REDIS_USER} on #${REDIS_HASH} ~train:* +@hash +@connection +@read" >> "$ACL_FILE"

if [ ! -f "$ACL_FILE" ]; then
    echo "CRITICAL: Failed to create ACL file at '$ACL_FILE'"
    exit 1
fi

echo "ACL file generated for user: ${REDIS_USER}"

# Hand off to redis
exec redis-server /usr/local/etc/redis/redis.conf