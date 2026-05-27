FROM node:22-alpine AS frontend-builder

WORKDIR /src/frontend

RUN corepack enable && corepack prepare pnpm@9.15.9 --activate
COPY frontend/package.json frontend/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile

COPY frontend/ ./
RUN pnpm build

FROM golang:1.22-alpine AS backend-builder

WORKDIR /src/backend

COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/sub2api-panel ./cmd/server

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=backend-builder /out/sub2api-panel /app/sub2api-panel
COPY --from=frontend-builder /src/frontend/dist /app/public

EXPOSE 8088

ENTRYPOINT ["/app/sub2api-panel"]
CMD ["-config", "/app/config.yaml", "-static", "/app/public"]
