FROM node:20-alpine AS frontend-builder

WORKDIR /app/frontend

COPY frontend/package.json frontend/package-lock.json ./

RUN npm config set registry https://registry.npmmirror.com && \
    npm ci

COPY frontend/ ./

RUN npm run build

FROM golang:1.25-alpine AS backend-builder

ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_TIME=unknown

WORKDIR /app

COPY backend/go.mod backend/go.sum ./

ENV GOPROXY=https://goproxy.cn,direct

RUN go mod download

COPY backend/ ./

COPY --from=frontend-builder /app/backend/.dist ./.dist

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w -X 'main.Version=${VERSION}' -X 'main.Commit=${COMMIT}' -X 'main.BuildTime=${BUILD_TIME}'" -o rabbit-panel .

FROM alpine:latest

WORKDIR /app

RUN apk add --no-cache \
    docker-cli \
    docker-cli-compose \
    tzdata \
    ca-certificates && \
    mkdir -p /app/compose_projects /app/data/certs

ENV TZ=Asia/Shanghai

COPY --from=backend-builder /app/rabbit-panel .

# 3958: panel port (HTTPS when TLS_ENABLED=true); 3959: optional HTTP -> HTTPS redirect port
EXPOSE 3958 3959

CMD ["./rabbit-panel"]
