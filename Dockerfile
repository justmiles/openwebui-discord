# Build Stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY . /app

# Download dependencies
RUN go mod download

# Build the application statically
# Target the main package inside cmd/openwebui-discord
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/openwebui-discord ./cmd/openwebui-discord

# Final Stage
FROM scratch

# Copy the static binary from the builder stage
COPY --from=builder /app/openwebui-discord /openwebui-discord

# Copy configuration files
# Assuming the application looks for config in a relative 'configs' directory
COPY configs /configs

# Copy CA certificates from the builder stage
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Set the entrypoint
ENTRYPOINT ["/openwebui-discord"]