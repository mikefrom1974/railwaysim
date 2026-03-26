# build command (run from the rabbit directory and match version to change log):
# docker build --platform linux/amd64 --build-arg VERSION=0.1.0 -t kong-server:0.1.0 .

# ========== STAGE 1: binary build ========== 
FROM golang:1.26-alpine AS builder

# version info for build metadata; replaced with --build-arg VERSION=1.0.0 when building
ARG VERSION=development

# git / certs for fetching dependencies
RUN apk add --no-cache git ca-certificates

# build the app
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w -X main.Version=${VERSION}"

# ========== STAGE 2: final image ==========
FROM kong:3.9.1-ubuntu

# Install bash and openssl for the entrypoint scripts, gettext-base > envsubst for the yaml generation
USER root
RUN apt-get update && apt-get install -y bash openssl wget gettext-base && rm -rf /var/lib/apt/lists/*

# Install Optum community plugin (enterprise would use kafka-log)
RUN luarocks install kong-lua-resty-kafka
RUN luarocks install kong-kafka-log

# Install mtls-auth plugin (enterprise only; here we're going to offload mTLS to nginx)
# RUN luarocks install mtls-auth

# Clean up temp files
RUN rm -rf /tmp/luarocks*

# copy the binary from the builder stage
COPY --from=builder /app/kong-enroller /usr/local/bin/kong-enroller

# copy the entrypoint script and config template
COPY kong.yaml.template /etc/kong/kong.yaml.template
COPY entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh

# change entrypoint to our script
USER kong
ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]