# Copyright 2024 Wondermove Inc.
# SPDX-License-Identifier: Apache-2.0

# Build stage
FROM golang:1.22-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /app

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X main.version=0.109.0.1" \
    -o /otelcol-contrib \
    ./cmd/otelcol

# Final stage
FROM alpine:3.20

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN adduser -D -u 10001 otelcol

# Copy binary from builder
COPY --from=builder /otelcol-contrib /otelcol-contrib

# Use non-root user
USER otelcol

# Expose ports
# OTLP gRPC
EXPOSE 4317
# OTLP HTTP
EXPOSE 4318
# Health check
EXPOSE 13133
# zpages
EXPOSE 55679

ENTRYPOINT ["/otelcol-contrib"]
CMD ["--config=/etc/otelcol/config.yaml"]
