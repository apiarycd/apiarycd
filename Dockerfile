FROM golang:1.24-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/apiarycd .

FROM alpine:3.22.2

# Runtime dependencies:
# - ca-certificates/tzdata for TLS and timezone handling
# - docker-cli because the app shells out to `docker stack ...`
RUN apk add --no-cache ca-certificates tzdata docker-cli git

RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser --home /app appuser

WORKDIR /app

COPY --from=builder --chown=appuser:appuser /out/apiarycd ./server

USER appuser

ENTRYPOINT ["/app/server"]
