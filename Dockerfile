FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o server ./cmd/api/
RUN CGO_ENABLED=0 GOOS=linux go build -o healthcheck ./cmd/healthcheck/

FROM gcr.io/distroless/static-debian12

WORKDIR /app

COPY --from=builder /app/server /app/server
COPY --from=builder /app/migrations ./migrations
COPY --from=builder /app/healthcheck /app/healthcheck

EXPOSE 8080

CMD ["/app/server"]