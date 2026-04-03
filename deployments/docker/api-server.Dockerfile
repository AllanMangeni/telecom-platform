FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY apps/api-server/go.mod apps/api-server/go.sum ./
RUN go mod download

COPY apps/api-server/ ./

RUN CGO_ENABLED=0 GOOS=linux go build -o api-server .

FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

COPY --from=builder /app/api-server .

EXPOSE 8000

CMD ["./api-server"]
