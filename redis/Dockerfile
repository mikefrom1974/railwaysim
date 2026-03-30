# build command (run from the redis directory and match version to change log):
# docker build --platform linux/amd64 --build-arg VERSION=0.1.0 -t redis-server:0.1.0 .

# ========== STAGE 1: binary build ========== 
FROM redis:8.6.2-alpine

# Create config directory
RUN mkdir -p /usr/local/etc/redis && chown redis:redis /usr/local/etc/redis

# Copy the base config template
COPY redis.conf /usr/local/etc/redis/redis.conf

# Copy the entrypoint script that will generate the ACL file
COPY entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh

# Redis runs as 'redis' user, but entrypoint needs to write the ACL file
ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]