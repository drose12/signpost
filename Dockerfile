###############################################################################
# Stage 1: Build the frontend
###############################################################################
FROM node:25-alpine AS frontend

WORKDIR /build/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

###############################################################################
# Stage 2: Build the SignPost Go binary
###############################################################################
FROM golang:1.24-alpine AS builder

ARG VERSION=dev

RUN apk add --no-cache gcc musl-dev sqlite-dev

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download

COPY . .
# Copy built frontend into the Go build context for embed
COPY --from=frontend /build/web/dist web/dist/
RUN CGO_ENABLED=1 go build -o signpost ./cmd/signpost/

###############################################################################
# Stage 3: Final image based on Maddy
###############################################################################
FROM foxcpp/maddy:0.9.2

# Install s6-overlay for process management
ARG S6_OVERLAY_VERSION=3.2.0.2
ADD https://github.com/just-containers/s6-overlay/releases/download/v${S6_OVERLAY_VERSION}/s6-overlay-noarch.tar.xz /tmp
ADD https://github.com/just-containers/s6-overlay/releases/download/v${S6_OVERLAY_VERSION}/s6-overlay-x86_64.tar.xz /tmp
RUN tar -C / -Jxpf /tmp/s6-overlay-noarch.tar.xz && \
    tar -C / -Jxpf /tmp/s6-overlay-x86_64.tar.xz && \
    rm /tmp/s6-overlay-*.tar.xz

# Install runtime dependencies
RUN apk add --no-cache curl sqlite-libs openssl msmtp

# Copy SignPost binary and templates
COPY --from=builder /build/signpost /app/signpost
COPY templates/ /app/templates/

# Copy s6 service definitions
COPY rootfs/ /

# Create data directory
RUN mkdir -p /data/signpost/dkim_keys /data/signpost/tls /data/signpost/logs /data/signpost/backups

# Environment defaults
ENV SIGNPOST_DATA_DIR=/data/signpost \
    SIGNPOST_TEMPLATE_PATH=/app/templates/maddy.conf.tmpl \
    SIGNPOST_WEB_PORT=8080 \
    SIGNPOST_SMTP_PORT=25 \
    SIGNPOST_SUBMISSION_PORT=587 \
    SIGNPOST_LOG_LEVEL=info

# Expose ports
EXPOSE 25 587 8080

# Health check
HEALTHCHECK --interval=30s --timeout=5s --retries=3 \
    CMD curl -f http://localhost:8080/api/v1/healthz || exit 1

# s6-overlay entrypoint
ENTRYPOINT ["/init"]
