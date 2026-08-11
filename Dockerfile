FROM golang:1.24-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /configforge ./cmd/configforge
RUN CGO_ENABLED=0 go build -o /basic-api ./examples/basic-api

FROM alpine:3.20
RUN adduser -D -u 10001 appuser
COPY --from=builder /configforge /usr/local/bin/configforge
COPY --from=builder /basic-api /usr/local/bin/basic-api
COPY examples/configs/default.yaml /etc/configforge/default.yaml
USER appuser
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/basic-api", "--config", "/etc/configforge/default.yaml", "--addr", ":8080"]