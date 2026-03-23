# build command (run from the rabbit directory and match version to change log):
# docker build --platform linux/amd64 --build-arg VERSION=0.1.0 -t rabbit-server:0.1.0 .

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
FROM rabbitmq:3.11-management

# allow our cert to specify our username
RUN rabbitmq-plugins enable --offline rabbitmq_auth_mechanism_ssl

# install openssl and curl for health checks / debugging
RUN apt-get update && apt-get install -y openssl curl && rm -rf /var/lib/apt/lists/*

# copy the binary from the builder stage
COPY --from=builder /app/rabbit-enroller /usr/local/bin/rabbit-enroller

# copy the .conf file
COPY rabbitmq.conf /etc/rabbitmq/rabbitmq.conf

# copy the entrypoint script
COPY entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh

# change entrypoint to our script
ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]