FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /syslog-server .

FROM alpine:3.20

RUN adduser -D -H appuser

COPY --from=builder /syslog-server /syslog-server

USER appuser

ENTRYPOINT ["/syslog-server"]
