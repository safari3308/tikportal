# Stage 1: Build React Frontend
FROM oven/bun:1-alpine AS frontend-builder
WORKDIR /app/web
COPY web/package.json ./
RUN bun install
COPY web/ ./
RUN bun run build

# Stage 2: Build Go Backend (with embedded React dist)
FROM golang:1.24-alpine AS backend-builder
WORKDIR /app
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
COPY --from=frontend-builder /app/web/dist ./web/dist
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /tikportal ./cmd/tikportal

# Stage 3: Final Minimal Image
FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=backend-builder /tikportal /app/tikportal
VOLUME ["/app/data"]
EXPOSE 8080
ENV LISTEN_ADDR=":8080" \
    DB_PATH="/app/data/tikportal.db"
ENTRYPOINT ["/app/tikportal"]
