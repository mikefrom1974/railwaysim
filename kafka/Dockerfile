# build command (run from the kafka directory and match version to change log):
# docker build --platform linux/amd64 --build-arg VERSION=0.1.0 -t kafka-server:0.1.0 .

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
FROM apache/kafka:4.1.2

# Install bash and openssl for the entrypoint scripts
USER root
RUN apk add --no-cache bash openssl tcpdump

# copy the binary from the builder stage
COPY --from=builder /app/kafka-enroller /usr/local/bin/kafka-enroller

# copy the entrypoint script
COPY entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh

# change entrypoint to our script
USER 1000
ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]