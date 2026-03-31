# build command (run from the spog directory and match version to change log):
# docker build --platform linux/amd64 --build-arg VERSION=0.1.0 -t spog:0.1.0 .

# ========== STAGE 1: binary build ========== 
FROM golang:1.26-alpine AS builder

# version info for build metadata; replaced with --build-arg VERSION=1.0.0 when building
ARG VERSION=development

# git / certs for fetching dependencies
RUN apk add --no-cache git ca-certificates

# build the app
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w -X main.Version=${VERSION}" -o spog *.go

# ========== STAGE 2: final image ==========
FROM alpine:latest

# security: certs and non-root user
RUN apk add --no-cache ca-certificates tzdata && \
    addgroup -S spog && adduser -S spog -G spog

# copy the binary from the builder stage
WORKDIR /app
COPY --from=builder /app/spog .

# switch to unprivileged user
USER spog

# expose the port
EXPOSE 8080

# set the entrypoint to run the API
ENTRYPOINT ["/app/spog"]