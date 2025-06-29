# --- Stage 1: Frontend Builder ---
FROM node:20-alpine AS frontend-builder

WORKDIR /app/web

# Copy web project files
COPY web/package.json web/package-lock.json ./
COPY web/tsconfig.json web/tsconfig.node.json web/tsconfig.app.json ./
COPY web/vite.config.ts ./

# Install dependencies
RUN npm install

# Copy the rest of the web source code
COPY web/ ./

# Build the frontend application
RUN npm run build

# --- Stage 2: Backend Builder ---
FROM golang:1.22-alpine AS backend-builder

WORKDIR /app

# Install build tools
RUN apk add --no-cache git build-base

# Copy Go module files and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the entire Go project source code
COPY . .

# Copy the built frontend from the previous stage
COPY --from=frontend-builder /app/web/dist ./web/dist

# Build the Go application
# We use CGO_ENABLED=0 to create a static binary
# -ldflags="-w -s" strips debug information and symbols to reduce binary size
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s" \
    -o /gpt-load \
    ./cmd/gpt-load

# --- Stage 3: Final Image ---
FROM alpine:latest

# Install necessary runtime dependencies
# ca-certificates for HTTPS connections
# tzdata for time zone information
RUN apk --no-cache add ca-certificates tzdata

# Create a non-root user and group for security
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

# Set the working directory
WORKDIR /app

# Copy the compiled binary from the backend-builder stage
COPY --from=backend-builder /gpt-load .

# Copy the configuration file example
COPY .env.example .

# Set ownership of the app directory to the non-root user
RUN chown -R appuser:appgroup /app

# Switch to the non-root user
USER appuser

# Expose the application port
# This should match the port defined in the configuration
EXPOSE 8080

# Healthcheck to ensure the application is running
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD [ "wget", "-q", "--spider", "http://localhost:8080/health" ] || exit 1

# Set the entrypoint for the container
ENTRYPOINT ["/app/gpt-load"]
