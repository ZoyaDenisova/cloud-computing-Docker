FROM golang:1.22-alpine AS builder

WORKDIR /src

COPY app/src/go.mod app/src/go.sum ./
RUN go mod download

COPY app/src/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/todo-app ./cmd/server

FROM alpine:3.20

WORKDIR /app
COPY --from=builder /out/todo-app /app/todo-app

EXPOSE 8080

CMD ["/app/todo-app"]
