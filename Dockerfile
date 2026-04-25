# Build frontend
FROM node:25.9.0-alpine AS frontend-builder

RUN npm install -g pnpm

WORKDIR /app

COPY web/package.json web/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile

COPY web/ .
RUN pnpm build

# Build backend
FROM golang:1.26.2-alpine AS backend-builder

WORKDIR /app

RUN apk add --no-cache git gcc musl-dev

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Embed built frontend into binary
COPY --from=frontend-builder /app/build/client ./internal/web/dist

ARG VERSION=dev
RUN CGO_ENABLED=1 GOOS=linux go build -ldflags "-s -w -X main.version=${VERSION}" -o main cmd/api/main.go

# Final stage
FROM alpine:3.23

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

COPY --from=backend-builder /app/main .

RUN mkdir -p /app/data

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

CMD ["./main"]
