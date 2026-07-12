# --- Stage 1: build frontend ---
FROM node:22-alpine AS web
WORKDIR /app/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
ENV NODE_OPTIONS="--max-old-space-size=4096"
RUN npm run build

# --- Stage 2: build backend (pure Go, no cgo) ---
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /app/web/dist ./web/dist
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o mochi .

# --- Stage 3: runtime ---
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /app/mochi /usr/local/bin/mochi
ENV MOCHI_DATA=/data
VOLUME /data
EXPOSE 3000
ENTRYPOINT ["mochi"]
