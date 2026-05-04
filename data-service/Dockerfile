FROM golang:1.22-alpine AS builder

WORKDIR /src

COPY src/go.mod src/go.sum ./
RUN go mod download

COPY src/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/data-service ./cmd/server

FROM alpine:3.20

WORKDIR /app
COPY --from=builder /out/data-service /app/data-service

EXPOSE 8081

CMD ["/app/data-service"]
