# Build Stage
# Using golang:1.24-alpine for a balance of build speed and image size.
FROM golang:1.24-alpine AS builder

# Install necessary build dependencies. 
# git and ca-certificates are required for fetching private modules or making HTTPS requests.
# tzdata is needed for timezone-aware operations.
RUN apk update && apk add --no-cache git ca-certificates tzdata

# Security Best Practice: Create a non-privileged user and group.
# This ensures the final container doesn't run as root, reducing the attack surface.
ENV USER=appuser
ENV UID=10001
RUN adduser \
    --disabled-password \
    --gecos "" \
    --home "/nonexistent" \
    --shell "/sbin/nologin" \
    --no-create-home \
    --uid "${UID}" \
    "${USER}"

WORKDIR /build

# Leverage Docker layer caching by copying go.mod and go.sum first.
COPY go.mod go.sum ./
RUN go mod download

# Copy the entire source code.
COPY . .

# Build a statically linked binary.
# CGO_ENABLED=0 ensures the binary has no external C library dependencies.
# -ldflags="-w -s" strips debug symbols and DWARF tables to minimize binary size.
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o /build/market-aggregator ./cmd/app/main.go

# Production Stage
# Using 'scratch' as the base image for a minimal, zero-dependency environment.
# This contains ONLY our binary and essential system files.
FROM scratch

# Import essential system files from the builder stage.
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /etc/group /etc/group

# Copy our application executable.
COPY --from=builder /build/market-aggregator /market-aggregator

# Copy the OpenAPI specification for Swagger UI.
COPY --from=builder /build/api /api

# Switch to the non-root user defined in the builder stage.
USER appuser:appuser

# The application listens for external price data and serves an HTTP API.
ENTRYPOINT ["/market-aggregator"]
