# ── Stage 1: Build ─────────────────────────────────────────────
# Use the official Go image to compile your source code.
FROM golang:1.25-alpine AS builder
 
WORKDIR /app
 
# Copy dependency files FIRST so Docker caches this layer.
# If only your code changes, modules are NOT re-downloaded.
COPY go.mod go.sum ./
RUN go mod download
 
# Now copy the rest of your source code
COPY . .
 
# Build a static binary. CGO_ENABLED=0 removes C dependencies.
RUN CGO_ENABLED=0 GOOS=linux go build -o api ./cmd/api/main.go
 
# ── Stage 2: Run ────────────────────────────────────────────────
# Use a tiny Alpine image — only ~5MB vs ~300MB for the Go image.
# This is called a multi-stage build.
FROM alpine:latest
 
WORKDIR /app
 
# Copy ONLY the compiled binary from the builder stage
COPY --from=builder /app/api .
 
# Create the secrets directory for Firebase credentials
RUN mkdir -p secrets
 
EXPOSE 8080
 
CMD ["./api"]
