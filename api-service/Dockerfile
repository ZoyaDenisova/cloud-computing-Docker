FROM golang:1.22-alpine AS builder

WORKDIR /src

COPY src/go.mod src/go.sum ./
RUN go mod download

COPY src/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/api-service ./cmd/server

FROM alpine:3.20

WORKDIR /app
COPY --from=builder /out/api-service /app/api-service

EXPOSE 8080

CMD ["/app/api-service"]
