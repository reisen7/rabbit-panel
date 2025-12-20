FROM golang:1.25-alpine AS builder

WORKDIR /app

RUN apk add --no-cache gcc musl-dev

COPY go.mod go.sum ./

RUN go env -w GOPROXY=https://proxy.golang.org,direct

RUN go mod tidy

COPY . .

RUN CGO_ENABLED=1 go build -ldflags="-s -w" -o rabbit-panel .

FROM alpine:latest

WORKDIR /app

RUN apk add --no-cache \
    docker-cli \ 
    docker-cli-compose \  
    tzdata=2025c-r0 \
    ca-certificates=20251003-r0

ENV TZ=Asia/Shanghai

COPY --from=builder /app/rabbit-panel .

RUN mkdir -p /app/compose_projects

EXPOSE 9999

CMD ["./rabbit-panel"]
