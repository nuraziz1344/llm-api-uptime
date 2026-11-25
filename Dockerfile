# Stage 1: Build React SPA
FROM node:22-alpine AS web-builder
WORKDIR /web
RUN corepack enable && corepack prepare pnpm@latest --activate
COPY web/package.json web/pnpm-lock.yaml* ./
RUN pnpm install --no-frozen-lockfile --dangerously-allow-all-builds
COPY web/ ./
RUN pnpm build

# Stage 2: Build static Go binary with embedded UI
FROM golang:1.26-alpine AS go-builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ ./cmd/
COPY internal/ ./internal/
RUN rm -rf internal/webui/dist
COPY --from=web-builder /web/dist ./internal/webui/dist
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/llm-api-uptime ./cmd/server

# Stage 3: Minimal runtime image
FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata && \
    adduser -D -u 10001 appuser && \
    mkdir -p /data && chown appuser:appuser /data

WORKDIR /app
COPY --from=go-builder /out/llm-api-uptime /app/llm-api-uptime

USER appuser
ENV PORT=8080
ENV DATA_DIR=/data
EXPOSE 8080
VOLUME ["/data"]

ENTRYPOINT ["/app/llm-api-uptime"]
